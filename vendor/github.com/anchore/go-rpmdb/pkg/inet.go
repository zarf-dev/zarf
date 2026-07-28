package rpmdb

import (
	"encoding/binary"
)

func Htonl(val int32) int32 {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(val))
	return int32(binary.BigEndian.Uint32(buf[:]))
}

func HtonlU(val uint32) uint32 {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], val)
	return binary.BigEndian.Uint32(buf[:])
}
