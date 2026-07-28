package squashfslow

import (
	"errors"
	"io"

	"github.com/sylabs/squashfs/internal/metadata"
	"github.com/sylabs/squashfs/internal/toreader"
	"github.com/sylabs/squashfs/low/data"
	"github.com/sylabs/squashfs/low/directory"
	"github.com/sylabs/squashfs/low/inode"
)

type FileBase struct {
	Inode inode.Inode
	Name  string
}

func (r *Reader) BaseFromInode(i inode.Inode, name string) FileBase {
	return FileBase{Inode: i, Name: name}
}

func (r *Reader) BaseFromEntry(e directory.Entry) (FileBase, error) {
	in, err := r.InodeFromEntry(e)
	if err != nil {
		return FileBase{}, err
	}
	return FileBase{Inode: in, Name: e.Name}, nil
}

func (r *Reader) BaseFromRef(ref uint64, name string) (FileBase, error) {
	in, err := r.InodeFromRef(ref)
	if err != nil {
		return FileBase{}, err
	}
	return FileBase{Inode: in, Name: name}, nil
}

func (b *FileBase) Uid(r *Reader) (uint32, error) {
	return r.Id(b.Inode.UidInd)
}

func (b *FileBase) Gid(r *Reader) (uint32, error) {
	return r.Id(b.Inode.GidInd)
}

func (b *FileBase) IsDir() bool {
	return b.Inode.Type == inode.Dir || b.Inode.Type == inode.EDir
}

func (b *FileBase) ToDir(r *Reader) (Directory, error) {
	var blockStart uint32
	var size uint32
	var offset uint16
	switch b.Inode.Type {
	case inode.Dir:
		blockStart = b.Inode.Data.(inode.Directory).BlockStart
		size = uint32(b.Inode.Data.(inode.Directory).Size)
		offset = b.Inode.Data.(inode.Directory).Offset
	case inode.EDir:
		blockStart = b.Inode.Data.(inode.EDirectory).BlockStart
		size = b.Inode.Data.(inode.EDirectory).Size
		offset = b.Inode.Data.(inode.EDirectory).Offset
	default:
		return Directory{}, errors.New("not a directory")
	}
	dirRdr := metadata.NewReader(toreader.NewReader(r.r, int64(r.Superblock.DirTableStart)+int64(blockStart)), r.d)
	defer dirRdr.Close()
	_, err := dirRdr.Read(make([]byte, offset))
	if err != nil {
		return Directory{}, err
	}
	entries, err := directory.ReadDirectory(dirRdr, size)
	if err != nil {
		return Directory{}, err
	}
	return Directory{
		FileBase: *b,
		Entries:  entries,
	}, nil
}

func (b *FileBase) IsRegular() bool {
	return b.Inode.Type == inode.Fil || b.Inode.Type == inode.EFil
}

func (b *FileBase) GetRegFileReaders(r *Reader) (*data.Reader, *data.FullReader, error) {
	if !b.IsRegular() {
		return nil, nil, errors.New("not a regular file")
	}
	var blockStart uint64
	var fragIndex uint32
	var fragOffset uint32
	var fragSize uint64
	var sizes []uint32
	if b.Inode.Type == inode.Fil {
		blockStart = uint64(b.Inode.Data.(inode.File).BlockStart)
		fragIndex = b.Inode.Data.(inode.File).FragInd
		fragOffset = b.Inode.Data.(inode.File).FragOffset
		sizes = b.Inode.Data.(inode.File).BlockSizes
		fragSize = uint64(b.Inode.Data.(inode.File).Size % r.Superblock.BlockSize)
	} else {
		blockStart = b.Inode.Data.(inode.EFile).BlockStart
		fragIndex = b.Inode.Data.(inode.EFile).FragInd
		fragOffset = b.Inode.Data.(inode.EFile).FragOffset
		sizes = b.Inode.Data.(inode.EFile).BlockSizes
		fragSize = b.Inode.Data.(inode.EFile).Size % uint64(r.Superblock.BlockSize)
	}
	frag := func() (io.Reader, error) {
		ent, err := r.fragEntry(fragIndex)
		if err != nil {
			return nil, err
		}
		frag := data.NewReader(toreader.NewReader(r.r, int64(ent.Start)), r.d, []uint32{ent.Size}, uint64(r.Superblock.BlockSize), r.Superblock.BlockSize)
		frag.Read(make([]byte, fragOffset))
		return io.LimitReader(frag, int64(fragSize)), nil
	}
	outRdr := data.NewReader(toreader.NewReader(r.r, int64(blockStart)), r.d, sizes, fragSize, r.Superblock.BlockSize)
	if fragIndex != 0xffffffff {
		f, err := frag()
		if err != nil {
			return nil, nil, err
		}
		outRdr.AddFrag(f)
	}
	outFull := data.NewFullReader(r.r, int64(blockStart), r.d, sizes, fragSize, r.Superblock.BlockSize)
	if fragIndex != 0xffffffff {
		outFull.AddFrag(frag)
	}
	return outRdr, outFull, nil
}

func (b *FileBase) GetFullReader(r *Reader) (*data.FullReader, error) {
	if !b.IsRegular() {
		return nil, errors.New("not a regular file")
	}
	var blockStart uint64
	var fragIndex uint32
	var fragOffset uint32
	var fragSize uint64
	var sizes []uint32
	if b.Inode.Type == inode.Fil {
		blockStart = uint64(b.Inode.Data.(inode.File).BlockStart)
		fragIndex = b.Inode.Data.(inode.File).FragInd
		fragOffset = b.Inode.Data.(inode.File).FragOffset
		sizes = b.Inode.Data.(inode.File).BlockSizes
		fragSize = uint64(b.Inode.Data.(inode.File).Size % r.Superblock.BlockSize)
	} else {
		blockStart = b.Inode.Data.(inode.EFile).BlockStart
		fragIndex = b.Inode.Data.(inode.EFile).FragInd
		fragOffset = b.Inode.Data.(inode.EFile).FragOffset
		sizes = b.Inode.Data.(inode.EFile).BlockSizes
		fragSize = b.Inode.Data.(inode.EFile).Size % uint64(r.Superblock.BlockSize)
	}
	outFull := data.NewFullReader(r.r, int64(blockStart), r.d, sizes, fragSize, r.Superblock.BlockSize)
	if fragIndex != 0xffffffff {
		outFull.AddFrag(func() (io.Reader, error) {
			ent, err := r.fragEntry(fragIndex)
			if err != nil {
				return nil, err
			}
			frag := data.NewReader(toreader.NewReader(r.r, int64(ent.Start)), r.d, []uint32{ent.Size}, uint64(r.Superblock.BlockSize), r.Superblock.BlockSize)
			frag.Read(make([]byte, fragOffset))
			return io.LimitReader(frag, int64(fragSize)), nil
		})
	}
	return outFull, nil
}

func (b *FileBase) GetReader(r *Reader) (*data.Reader, error) {
	if !b.IsRegular() {
		return nil, errors.New("not a regular file")
	}
	var blockStart uint64
	var fragIndex uint32
	var fragOffset uint32
	var fragSize uint64
	var sizes []uint32
	if b.Inode.Type == inode.Fil {
		blockStart = uint64(b.Inode.Data.(inode.File).BlockStart)
		fragIndex = b.Inode.Data.(inode.File).FragInd
		fragOffset = b.Inode.Data.(inode.File).FragOffset
		sizes = b.Inode.Data.(inode.File).BlockSizes
		fragSize = uint64(b.Inode.Data.(inode.File).Size % r.Superblock.BlockSize)
	} else {
		blockStart = b.Inode.Data.(inode.EFile).BlockStart
		fragIndex = b.Inode.Data.(inode.EFile).FragInd
		fragOffset = b.Inode.Data.(inode.EFile).FragOffset
		sizes = b.Inode.Data.(inode.EFile).BlockSizes
		fragSize = b.Inode.Data.(inode.EFile).Size % uint64(r.Superblock.BlockSize)
	}
	outRdr := data.NewReader(toreader.NewReader(r.r, int64(blockStart)), r.d, sizes, fragSize, r.Superblock.BlockSize)
	if fragIndex != 0xffffffff {
		ent, err := r.fragEntry(fragIndex)
		if err != nil {
			return nil, err
		}
		frag := data.NewReader(toreader.NewReader(r.r, int64(ent.Start)), r.d, []uint32{ent.Size}, uint64(r.Superblock.BlockSize), r.Superblock.BlockSize)
		frag.Read(make([]byte, fragOffset))
		outRdr.AddFrag(io.LimitReader(frag, int64(fragSize)))
	}
	return outRdr, nil
}
