package inode

import (
	"encoding/binary"
	"io"
	"math"
)

type fileInit struct {
	BlockStart uint32
	FragInd    uint32
	FragOffset uint32
	Size       uint32
}

type File struct {
	fileInit
	BlockSizes []uint32
}

type eFileInit struct {
	BlockStart uint64
	Size       uint64
	Sparse     uint64
	LinkCount  uint32
	FragInd    uint32
	FragOffset uint32
	XattrInd   uint32
}

type EFile struct {
	eFileInit
	BlockSizes []uint32
}

func ReadFile(r io.Reader, blockSize uint32) (f File, err error) {
	err = binary.Read(r, binary.LittleEndian, &f.fileInit)
	if err != nil {
		return
	}
	toRead := int(math.Floor(float64(f.Size) / float64(blockSize)))
	if f.FragInd == 0xFFFFFFFF && f.Size%blockSize > 0 {
		toRead++
	}
	f.BlockSizes = make([]uint32, toRead)
	err = binary.Read(r, binary.LittleEndian, &f.BlockSizes)
	return
}

func ReadEFile(r io.Reader, blockSize uint32) (f EFile, err error) {
	err = binary.Read(r, binary.LittleEndian, &f.eFileInit)
	if err != nil {
		return
	}
	toRead := int(math.Floor(float64(f.Size) / float64(blockSize)))
	if f.FragInd == 0xFFFFFFFF && f.Size%uint64(blockSize) > 0 {
		toRead++
	}
	f.BlockSizes = make([]uint32, toRead)
	err = binary.Read(r, binary.LittleEndian, &f.BlockSizes)
	return
}
