package inode

import (
	"encoding/binary"
	"io"
)

type Device struct {
	LinkCount uint32
	Dev       uint32
}

type EDevice struct {
	Device
	XattrInd uint32
}

func ReadDevice(r io.Reader) (d Device, err error) {
	err = binary.Read(r, binary.LittleEndian, &d)
	return
}

func ReadEDevice(r io.Reader) (d EDevice, err error) {
	err = binary.Read(r, binary.LittleEndian, &d)
	return
}

type IPC struct {
	LinkCount uint32
}

type EIPC struct {
	IPC
	XattrInd uint32
}

func ReadIPC(r io.Reader) (i IPC, err error) {
	err = binary.Read(r, binary.LittleEndian, &i)
	return
}

func ReadEIPC(r io.Reader) (i EIPC, err error) {
	err = binary.Read(r, binary.LittleEndian, &i)
	return
}
