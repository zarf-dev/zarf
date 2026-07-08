package squashfslow

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sylabs/squashfs/internal/metadata"
	"github.com/sylabs/squashfs/internal/toreader"
	"github.com/sylabs/squashfs/low/directory"
	"github.com/sylabs/squashfs/low/inode"
)

type Directory struct {
	FileBase
	Entries []directory.Entry
}

func (r *Reader) directoryFromRef(ref uint64, name string) (Directory, error) {
	i, err := r.InodeFromRef(ref)
	if err != nil {
		return Directory{}, err
	}
	var blockStart uint32
	var size uint32
	var offset uint16
	switch i.Type {
	case inode.Dir:
		blockStart = i.Data.(inode.Directory).BlockStart
		size = uint32(i.Data.(inode.Directory).Size)
		offset = i.Data.(inode.Directory).Offset
	case inode.EDir:
		blockStart = i.Data.(inode.EDirectory).BlockStart
		size = i.Data.(inode.EDirectory).Size
		offset = i.Data.(inode.EDirectory).Offset
	default:
		return Directory{}, errors.New("not a directory")
	}
	dirRdr := metadata.NewReader(toreader.NewReader(r.r, int64(r.Superblock.DirTableStart)+int64(blockStart)), r.d)
	defer dirRdr.Close()
	_, err = dirRdr.Read(make([]byte, offset))
	if err != nil {
		return Directory{}, err
	}
	entries, err := directory.ReadDirectory(dirRdr, size)
	if err != nil {
		return Directory{}, err
	}
	return Directory{
		FileBase: r.BaseFromInode(i, name),
		Entries:  entries,
	}, nil
}

func (d *Directory) Open(r *Reader, path string) (FileBase, error) {
	path = filepath.Clean(path)
	if path == "." || path == "" {
		return d.FileBase, nil
	}
	split := strings.Split(path, "/")
	i, found := slices.BinarySearchFunc(d.Entries, split[0], func(e directory.Entry, name string) int {
		return strings.Compare(e.Name, name)
	})
	if !found {
		return FileBase{}, fs.ErrNotExist
	}
	b, err := r.BaseFromEntry(d.Entries[i])
	if err != nil {
		return FileBase{}, err
	}
	if len(split) == 1 {
		return b, nil
	} else if !b.IsDir() {
		return FileBase{}, fs.ErrNotExist
	}
	dir, err := b.ToDir(r)
	if err != nil {
		return FileBase{}, err
	}
	return dir.Open(r, strings.Join(split[1:], "/"))
}
