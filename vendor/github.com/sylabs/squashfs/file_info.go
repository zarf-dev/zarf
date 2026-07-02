package squashfs

import (
	"io/fs"
	"time"

	"github.com/sylabs/squashfs/low/directory"
	"github.com/sylabs/squashfs/low/inode"
)

type fileInfo struct {
	name     string
	size     int64
	perm     uint32
	modTime  uint32
	fileType uint16
}

func (r Reader) newFileInfo(e directory.Entry) (fileInfo, error) {
	i, err := r.Low.InodeFromEntry(e)
	if err != nil {
		return fileInfo{}, err
	}
	return newFileInfo(e.Name, &i), nil
}

func newFileInfo(name string, i *inode.Inode) fileInfo {
	var size int64
	switch i.Type {
	case inode.Fil:
		size = int64(i.Data.(inode.File).Size)
	case inode.EFil:
		size = int64(i.Data.(inode.EFile).Size)
	}
	return fileInfo{
		name:     name,
		size:     size,
		perm:     uint32(i.Perm),
		modTime:  i.ModTime,
		fileType: i.Type,
	}
}

func (f fileInfo) Name() string {
	return f.name
}

func (f fileInfo) Size() int64 {
	return f.size
}

func (f fileInfo) Mode() fs.FileMode {
	switch f.fileType {
	case inode.Dir, inode.EDir:
		return fs.FileMode(f.perm | uint32(fs.ModeDir))
	case inode.Sym, inode.ESym:
		return fs.FileMode(f.perm | uint32(fs.ModeSymlink))
	case inode.Char, inode.EChar, inode.Block, inode.EBlock:
		return fs.FileMode(f.perm | uint32(fs.ModeDevice))
	case inode.Fifo, inode.EFifo:
		return fs.FileMode(f.perm | uint32(fs.ModeNamedPipe))
	case inode.Sock, inode.ESock:
		return fs.FileMode(f.perm | uint32(fs.ModeSocket))
	}
	return fs.FileMode(f.perm)
}

func (f fileInfo) ModTime() time.Time {
	return time.Unix(int64(f.modTime), 0)
}

func (f fileInfo) IsDir() bool {
	return f.fileType == inode.Dir || f.fileType == inode.EDir
}

func (f fileInfo) IsSymlink() bool {
	return f.fileType == inode.Sym || f.fileType == inode.ESym
}

func (f fileInfo) IsDevice() bool {
	return f.fileType == inode.Block || f.fileType == inode.EBlock ||
		f.fileType == inode.Char || f.fileType == inode.EChar
}

func (f fileInfo) IsFifo() bool {
	return f.fileType == inode.Fifo || f.fileType == inode.EFifo
}

func (f fileInfo) IsSocket() bool {
	return f.fileType == inode.Sock || f.fileType == inode.ESock
}

func (f fileInfo) Sys() any {
	return nil
}
