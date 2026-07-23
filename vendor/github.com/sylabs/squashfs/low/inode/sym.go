package inode

import (
	"encoding/binary"
	"io"
)

type symlinkInit struct {
	LinkCount  uint32
	TargetSize uint32
}

type Symlink struct {
	symlinkInit
	Target []byte
}

type ESymlink struct {
	symlinkInit
	Target   []byte
	XattrInd uint32
}

func ReadSym(r io.Reader) (s Symlink, err error) {
	err = binary.Read(r, binary.LittleEndian, &s.symlinkInit)
	if err != nil {
		return
	}
	s.Target = make([]byte, s.TargetSize)
	err = binary.Read(r, binary.LittleEndian, &s.Target)
	return
}

func ReadESym(r io.Reader) (s ESymlink, err error) {
	err = binary.Read(r, binary.LittleEndian, &s.symlinkInit)
	if err != nil {
		return
	}
	s.Target = make([]byte, s.TargetSize)
	err = binary.Read(r, binary.LittleEndian, &s.Target)
	if err != nil {
		return
	}
	err = binary.Read(r, binary.LittleEndian, &s.XattrInd)
	return
}
