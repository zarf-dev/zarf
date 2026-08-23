package rardecode

import (
	"io"
	"math/bits"
)

type bitReader interface {
	readBits(n uint8) (int, error) // read n bits of data
	unreadBits(n uint8)            // revert the reading of the last n bits read
}

// rar5BitReader is a bitReader that reads bytes from a byteReader and stops with io.EOF after l bits.
type rar5BitReader struct {
	r byteReader
	v int   // cache of bits read from r
	l int   // number of bits (not cached) that can be read from r
	n uint8 // number of unread bits in v
}

func (r *rar5BitReader) unreadBits(n uint8) { r.n += n }

// ReadByte reads and returns a single byte. If no byte is available, returns an error.
func (r *rar5BitReader) ReadByte() (byte, error) {
	if r.n == 0 {
		return r.r.ReadByte()
	}
	b, err := r.readBits(8)
	return byte(b), err
}

func (r *rar5BitReader) reset(br byteReader) {
	r.r = br
}

// setLimit sets the maximum bit count that can be read.
func (r *rar5BitReader) setLimit(n int) {
	r.l = n
	r.n = 0
}

// readBits returns n bits from the underlying byteReader.
// n must be less than integer size - 8.
func (r *rar5BitReader) readBits(n uint8) (int, error) {
	for n > r.n {
		if r.l == 0 {
			// reached bits limit
			return 0, io.EOF
		}
		c, err := r.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				// io.EOF before we reached bit limit
				err = ErrDecoderOutOfData
			}
			return 0, err
		}
		r.v = r.v<<8 | int(c)
		r.n += 8
		r.l -= 8
		if r.l < 0 {
			// overshot, discard the extra bits
			bits := uint8(-r.l)
			r.l = 0
			r.v >>= bits
			r.n -= bits
		}
	}
	r.n -= n
	return (r.v >> r.n) & ((1 << n) - 1), nil
}

// rarBitReader wraps an io.ByteReader to perform various bit and byte
// reading utility functions used in RAR file processing.
type rarBitReader struct {
	r byteReader
	v int
	n uint8
}

func (r *rarBitReader) reset(br byteReader) {
	r.r = br
	r.n = 0
	r.v = 0
}

// readBits returns n bits from the underlying byteReader.
// n must be less than integer size - 8.
func (r *rarBitReader) readBits(n uint8) (int, error) {
	for n > r.n {
		b, err := r.r.ReadByte()
		if err != nil {
			return 0, err
		}
		r.v = r.v<<8 | int(b)
		r.n += 8
	}
	r.n -= n
	return (r.v >> r.n) & ((1 << n) - 1), nil
}

func (r *rarBitReader) unreadBits(n uint8) {
	r.n += n
}

// alignByte aligns the current bit reading input to the next byte boundary.
func (r *rarBitReader) alignByte() {
	r.n -= r.n % 8
}

// readUint32 reads a RAR V3 encoded uint32
func (r *rarBitReader) readUint32() (uint32, error) {
	n, err := r.readBits(2)
	if err != nil {
		return 0, err
	}
	if n != 1 {
		if bits.UintSize == 32 {
			if n == 3 {
				// 32bit platforms may not be able to read 32 bits as r.v
				// will need up to 7 extra bits for overflow from reading a byte.
				// Split it into two reads.
				n, err = r.readBits(16)
				if err != nil {
					return 0, err
				}
				m := uint32(n) << 16
				n, err = r.readBits(16)
				return m | uint32(n), err
			}
		}
		n, err = r.readBits(4 << uint(n))
		return uint32(n), err
	}
	n, err = r.readBits(4)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		n, err = r.readBits(8)
		n |= -1 << 8
		return uint32(n), err
	}
	nlow, err := r.readBits(4)
	n = n<<4 | nlow
	return uint32(n), err
}

func (r *rarBitReader) ReadByte() (byte, error) {
	if r.n == 0 {
		return r.r.ReadByte()
	}
	b, err := r.readBits(8)
	return byte(b), err
}

func (r *rarBitReader) Read(p []byte) (int, error) {
	if r.n == 0 {
		return r.r.Read(p)
	}
	for i := range p {
		b, err := r.readBits(8)
		if err != nil {
			return i, err
		}
		p[i] = byte(b)
	}
	return len(p), nil
}

func newRarBitReader(r byteReader) *rarBitReader {
	return &rarBitReader{r: r}
}
