package squashfslow

import (
	"github.com/sylabs/squashfs/internal/metadata"
	"github.com/sylabs/squashfs/internal/toreader"
	"github.com/sylabs/squashfs/low/directory"
	"github.com/sylabs/squashfs/low/inode"
)

func (r *Reader) InodeFromRef(ref uint64) (inode.Inode, error) {
	offset, meta := (ref>>16)+r.Superblock.InodeTableStart, ref&0xFFFF
	rdr := metadata.NewReader(toreader.NewReader(r.r, int64(offset)), r.d)
	defer rdr.Close()
	_, err := rdr.Read(make([]byte, meta))
	if err != nil {
		return inode.Inode{}, err
	}
	return inode.Read(rdr, r.Superblock.BlockSize)
}

func (r *Reader) InodeFromEntry(e directory.Entry) (inode.Inode, error) {
	rdr := metadata.NewReader(toreader.NewReader(r.r, int64(r.Superblock.InodeTableStart)+int64(e.BlockStart)), r.d)
	defer rdr.Close()
	rdr.Read(make([]byte, e.Offset))
	return inode.Read(rdr, r.Superblock.BlockSize)
}
