package inode

import (
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"strconv"
)

const (
	Dir = uint16(iota + 1)
	Fil
	Sym
	Block
	Char
	Fifo
	Sock
	EDir
	EFil
	ESym
	EBlock
	EChar
	EFifo
	ESock
)

type Header struct {
	Type    uint16
	Perm    uint16
	UidInd  uint16
	GidInd  uint16
	ModTime uint32
	Num     uint32
}

type Inode struct {
	Header
	Data any
}

func Read(r io.Reader, blockSize uint32) (i Inode, err error) {
	err = binary.Read(r, binary.LittleEndian, &i.Header)
	if err != nil {
		return
	}
	switch i.Type {
	case Dir:
		i.Data, err = ReadDir(r)
	case Fil:
		i.Data, err = ReadFile(r, blockSize)
	case Sym:
		i.Data, err = ReadSym(r)
	case Block:
		fallthrough
	case Char:
		i.Data, err = ReadDevice(r)
	case Fifo:
		fallthrough
	case Sock:
		i.Data, err = ReadIPC(r)
	case EDir:
		i.Data, err = ReadEDir(r)
	case EFil:
		i.Data, err = ReadEFile(r, blockSize)
	case ESym:
		i.Data, err = ReadESym(r)
	case EBlock:
		fallthrough
	case EChar:
		i.Data, err = ReadEDevice(r)
	case EFifo:
		fallthrough
	case ESock:
		i.Data, err = ReadEIPC(r)
	default:
		return i, errors.New("invalid inode type " + strconv.Itoa(int(i.Type)))
	}
	return
}

func (i Inode) Mode() (out fs.FileMode) {
	out = fs.FileMode(i.Perm)
	switch i.Type {
	case Dir, EDir:
		out |= fs.ModeDir
	case Sym, ESym:
		out |= fs.ModeSymlink
	case Char, EChar, Block, EBlock:
		out |= fs.ModeDevice
	case Fifo, EFifo:
		out |= fs.ModeNamedPipe
	case Sock, ESock:
		out |= fs.ModeSocket
	}
	return
}

func (i Inode) LinkCount() uint32 {
	switch i.Data.(type) {
	case EFile:
		return i.Data.(EFile).LinkCount
	case Directory:
		return i.Data.(Directory).LinkCount
	case EDirectory:
		return i.Data.(EDirectory).LinkCount
	case Device:
		return i.Data.(Device).LinkCount
	case EDevice:
		return i.Data.(EDevice).LinkCount
	case IPC:
		return i.Data.(IPC).LinkCount
	case EIPC:
		return i.Data.(EIPC).LinkCount
	case Symlink:
		return i.Data.(Symlink).LinkCount
	case ESymlink:
		return i.Data.(ESymlink).LinkCount
	default:
		return 0
	}
}

func (i Inode) Size() uint64 {
	switch i.Data.(type) {
	case File:
		return uint64(i.Data.(File).Size)
	case EFile:
		return i.Data.(EFile).Size
	// case Directory:
	// 	return uint64(i.Data.(Directory).Size)
	// case EDirectory:
	// 	return uint64(i.Data.(EDirectory).Size)
	default:
		return 0
	}
}
