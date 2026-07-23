package metadata

import (
	"encoding/binary"
	"io"

	"github.com/sylabs/squashfs/internal/decompress"
)

type Reader struct {
	r         io.Reader
	d         decompress.Decompressor
	dat       []byte
	curOffset uint16
}

func NewReader(r io.Reader, d decompress.Decompressor) *Reader {
	return &Reader{
		r: r,
		d: d,
	}
}

func (r *Reader) advance() error {
	r.curOffset = 0
	var size uint16
	err := binary.Read(r.r, binary.LittleEndian, &size)
	if err != nil {
		return err
	}
	realSize := size &^ 0x8000
	r.dat = make([]byte, realSize)
	err = binary.Read(r.r, binary.LittleEndian, &r.dat)
	if err != nil {
		return err
	}
	if size != realSize {
		return nil
	}
	r.dat, err = r.d.Decompress(r.dat)
	return err
}

func (r *Reader) Read(b []byte) (int, error) {
	curRead := 0
	var toRead int
	for curRead < len(b) {
		if r.curOffset >= uint16(len(r.dat)) {
			if err := r.advance(); err != nil {
				return curRead, err
			}
		}
		toRead = min(len(b)-curRead, len(r.dat)-int(r.curOffset))
		copy(b[curRead:], r.dat[r.curOffset:int(r.curOffset)+toRead])
		r.curOffset += uint16(toRead)
		curRead += toRead
	}
	return curRead, nil
}

func (r *Reader) Close() error {
	r.dat = nil
	return nil
}
