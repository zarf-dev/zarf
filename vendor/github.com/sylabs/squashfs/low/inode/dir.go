package inode

import (
	"encoding/binary"
	"io"
)

type Directory struct {
	BlockStart uint32
	LinkCount  uint32
	Size       uint16
	Offset     uint16
	ParentNum  uint32
}

type eDirectoryInit struct {
	LinkCount  uint32
	Size       uint32
	BlockStart uint32
	ParentNum  uint32
	IndCount   uint16
	Offset     uint16
	XattrInd   uint32
}

type EDirectory struct {
	eDirectoryInit
	Indexes []DirectoryIndex
}

type directoryIndexInit struct {
	Ind      uint32
	Start    uint32
	NameSize uint32
}

type DirectoryIndex struct {
	directoryIndexInit
	Name []byte
}

func ReadDir(r io.Reader) (d Directory, err error) {
	err = binary.Read(r, binary.LittleEndian, &d)
	return
}

func ReadEDir(r io.Reader) (d EDirectory, err error) {
	err = binary.Read(r, binary.LittleEndian, &d.eDirectoryInit)
	if err != nil {
		return
	}
	d.Indexes = make([]DirectoryIndex, d.IndCount)
	for i := range d.Indexes {
		err = binary.Read(r, binary.LittleEndian, &d.Indexes[i].directoryIndexInit)
		if err != nil {
			return
		}
		d.Indexes[i].Name = make([]byte, d.Indexes[i].NameSize+1)
		err = binary.Read(r, binary.LittleEndian, &d.Indexes[i].Name)
		if err != nil {
			return
		}
	}
	return
}
