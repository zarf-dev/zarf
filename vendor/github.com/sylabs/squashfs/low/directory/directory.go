package directory

import (
	"encoding/binary"
	"io"
)

type header struct {
	Count      uint32
	BlockStart uint32
	Num        uint32
}

type decEntry struct {
	Offset    uint16
	NumOffset int16
	InodeType uint16
	NameSize  uint16
	// Name []byte (not decoded along with decEntry)
}

type Entry struct {
	Name       string
	BlockStart uint32
	Offset     uint16
	InodeType  uint16
	Num        uint32
}

func ReadDirectory(r io.Reader, size uint32) (out []Entry, err error) {
	size -= 3
	var curRead uint32
	var h header
	var de decEntry
	for curRead < size {
		err = binary.Read(r, binary.LittleEndian, &h)
		if err != nil {
			return
		}
		curRead += 12
		for i := uint32(0); i < h.Count+1 && curRead < size; i++ {
			err = binary.Read(r, binary.LittleEndian, &de)
			if err != nil {
				return
			}
			nameTmp := make([]byte, de.NameSize+1)
			err = binary.Read(r, binary.LittleEndian, &nameTmp)
			if err != nil {
				return
			}
			curRead += 8 + uint32(de.NameSize) + 1
			out = append(out, Entry{
				BlockStart: h.BlockStart,
				Offset:     de.Offset,
				Name:       string(nameTmp),
				InodeType:  de.InodeType,
				Num:        h.Num + uint32(de.NumOffset),
			})
		}
	}
	return
}
