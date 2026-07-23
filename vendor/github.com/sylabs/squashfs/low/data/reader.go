package data

import (
	"encoding/binary"
	"io"

	"github.com/sylabs/squashfs/internal/decompress"
)

type Reader struct {
	r              io.Reader
	d              decompress.Decompressor
	frag           io.Reader
	sizes          []uint32
	dat            []byte
	curOffset      int
	curIndex       uint64
	finalBlockSize uint64
	blockSize      uint32
}

func NewReader(r io.Reader, d decompress.Decompressor, sizes []uint32, finalBlockSize uint64, blockSize uint32) *Reader {
	return &Reader{
		r:              r,
		d:              d,
		sizes:          sizes,
		finalBlockSize: finalBlockSize,
		blockSize:      blockSize,
	}
}

func (r *Reader) AddFrag(fragRdr io.Reader) {
	r.frag = fragRdr
}

func (r *Reader) advance() error {
	r.curOffset = 0
	defer func() { r.curIndex++ }()
	var err error
	if r.curIndex == uint64(len(r.sizes)) && r.frag != nil {
		r.dat, err = io.ReadAll(r.frag)
		return err
	} else if r.curIndex >= uint64(len(r.sizes)) {
		r.dat = []byte{}
		return io.EOF
	}
	realSize := r.sizes[r.curIndex] &^ (1 << 24)
	if realSize == 0 {
		if r.curIndex == uint64(len(r.sizes))-1 && r.frag == nil {
			r.dat = make([]byte, r.finalBlockSize)
		} else {
			r.dat = make([]byte, r.blockSize)
		}
		return nil
	}
	r.dat = make([]byte, realSize)
	err = binary.Read(r.r, binary.LittleEndian, &r.dat)
	if err != nil {
		return err
	}
	if r.sizes[r.curIndex] != realSize {
		return nil
	}
	r.dat, err = r.d.Decompress(r.dat)
	return err
}

func (r *Reader) Read(b []byte) (int, error) {
	curRead := 0
	var toRead int
	for curRead < len(b) {
		if r.curOffset >= len(r.dat) {
			if err := r.advance(); err != nil {
				return curRead, err
			}
		}
		toRead = min(len(b)-curRead, len(r.dat)-r.curOffset)
		toRead = copy(b[curRead:], r.dat[r.curOffset:r.curOffset+toRead])
		r.curOffset += toRead
		curRead += toRead
	}
	return curRead, nil
}

func (r *Reader) Close() error {
	if r.frag != nil {
		if l, ok := r.frag.(*io.LimitedReader); ok {
			if cl, ok := l.R.(io.Closer); ok {
				cl.Close()
			}
		}
	}
	r.dat = nil
	return nil
}
