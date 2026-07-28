/*
Package lzo is a direct implementation of the LZO decompression algorithm in Go using the following sources as references:
  - https://docs.kernel.org/staging/lzo.html (linux kernel documentation)
  - https://github.com/AxioDL/lzokay/blob/db2df1fcbebc2ed06c10f727f72567d40f06a2be/lzokay.cpp (lzokay C++ implementation, MIT licensed)
*/
package lzo

import (
	"encoding/binary"
	"errors"
	"runtime"
)

const (
	hostBigEndian = runtime.GOARCH == "ppc64" || runtime.GOARCH == "s390x" || runtime.GOARCH == "mips" || runtime.GOARCH == "mips64"
	max255Count   = (^uint(0))/255 - 2
	m3Marker      = 0x20
	m4Marker      = 0x10
)

var (
	ErrLookbehindOverrun   = errors.New("lzo: lookbehind overrun")
	ErrOutputOverrun       = errors.New("lzo: output overrun")
	ErrInputOverrun        = errors.New("lzo: input overrun")
	ErrDecompressionFailed = errors.New("lzo: error during decompression")
	ErrInputNotConsumed    = errors.New("lzo: input not fully consumed")
)

// decoder holds all state needed during decompression
type decoder struct {
	src, dst            buffer // source and destination buffers
	curState, nextState int    // current state and next state
	lbIdx, lbLen        int    // lookbehind current index and length
}

// buffer holds a slice and its current index and end boundary
type buffer struct {
	data     []byte // underlying data slice
	idx, end int    // current index and end of the buffer
}

func newDecoder(srcBytes, dstBytes []byte) *decoder {
	return &decoder{
		src: newBuffer(srcBytes),
		dst: newBuffer(dstBytes),
	}
}

func newBuffer(data []byte) buffer {
	return buffer{
		data: data,
		idx:  0,
		end:  len(data),
	}
}

// Decompress the given LZO1X compressed data to the destination buffer.
//
// Here's a summary of the state machine:
//
//	Instruction    Bits        Description
//	-------------- ------------ -------------------------------------------------
//	M1 (short)     0x00..0x0F  copy 2-3 bytes based on previous literal state
//	M1 (long)      0x00..0x0F  when state==0, long literal run (>=4 bytes)
//	M2             0x40..0xFF  copy 3-8 bytes within 2kB distance
//	M3             0x20..0x3F  copy small block within 16kB distance
//	M4             0x10..0x1F  copy block within 16..48kB, end-of-stream if distance==16384
func Decompress(srcBytes, dstBytes []byte) (outSize int, err error) {
	d := newDecoder(srcBytes, dstBytes)

	if d.src.end < 3 {
		return 0, ErrInputOverrun
	}

	if err = d.handleFirstByteEncoding(); err != nil {
		return d.dst.idx, err
	}

instructionLoop:
	for {
		if d.src.idx+1 > d.src.end {
			return d.dst.idx, ErrInputOverrun
		}
		inst := d.src.data[d.src.idx]
		d.src.idx++
		switch {
		case inst&0xC0 != 0:
			if err = d.handleM2(inst); err != nil {
				return d.dst.idx, err
			}
		case inst&m3Marker != 0:
			if err = d.handleM3(inst); err != nil {
				return d.dst.idx, err
			}
		case inst&m4Marker != 0:
			var finished bool
			if finished, err = d.handleM4(inst); err != nil {
				return d.dst.idx, err
			}
			if finished {
				break instructionLoop /* stream finished */
			}
		default:
			/* [M1] Depends on the number of literals copied by the last instruction. */
			switch d.curState {
			case 0:
				if err = d.handleM1LongLiteral(inst); err != nil {
					return d.dst.idx, err
				}
				d.curState = 4
				continue
			default:
				if err = d.handleM1ShortCopy(inst); err != nil {
					return d.dst.idx, err
				}
			}
		}

		if d.lbIdx < 0 {
			return d.dst.idx, ErrLookbehindOverrun
		}
		if d.src.idx+d.nextState > d.src.end {
			return d.dst.idx, ErrInputOverrun
		}
		if d.dst.idx+d.lbLen+d.nextState > d.dst.end {
			return d.dst.idx, ErrOutputOverrun
		}

		d.copyLookbehind()
		d.copyLiterals(d.nextState)
	}

	switch {
	case d.lbLen != 3:
		/* ensure terminating M4 was encountered */
		return d.dst.idx, ErrDecompressionFailed
	case d.src.idx == d.src.end:
		return d.dst.idx, nil
	case d.src.idx < d.src.end:
		return d.dst.idx, ErrInputNotConsumed
	default:
		return d.dst.idx, ErrInputOverrun
	}
}

//go:inline
func (d *decoder) handleFirstByteEncoding() error {
	switch {
	case d.src.data[d.src.idx] >= 22:
		/* 22..255 : copy literal string
		 *           length = (byte - 17) = 4..238
		 *           state = 4 [ don't copy extra literals ]
		 *           skip byte
		 */
		length := int(d.src.data[d.src.idx]) - 17
		d.src.idx++
		if d.src.idx+length > d.src.end {
			return ErrInputOverrun
		}
		if d.dst.idx+length > d.dst.end {
			return ErrOutputOverrun
		}
		d.copyLiterals(length)
		d.curState = 4

	case d.src.data[d.src.idx] >= 18:
		/* 18..21 : copy 0..3 literals
		 *          state = (byte - 17) = 0..3  [ copy <state> literals ]
		 *          skip byte
		 */
		length := int(d.src.data[d.src.idx]) - 17
		d.src.idx++
		if d.src.idx+length > d.src.end {
			return ErrInputOverrun
		}
		if d.dst.idx+length > d.dst.end {
			return ErrOutputOverrun
		}
		d.copyLiterals(length)
		d.curState = length
	}
	/* 0..17 : follow regular instruction encoding, see below. It is worth
	 *         noting that codes 16 and 17 will represent a block copy from
	 *         the dictionary which is empty, and that they will always be
	 *         invalid at this place.
	 */
	return nil
}

//go:inline
func (d *decoder) handleM2(inst byte) error {
	/* [M2]
	 * 1 L L D D D S S  (128..255)
	 *   Copy 5-8 bytes from block within 2kB distance
	 *   state = S (copy S literals after this block)
	 *   length = 5 + L
	 * Always followed by exactly one byte : H H H H H H H H
	 *   distance = (H << 3) + D + 1
	 *
	 * 0 1 L D D D S S  (64..127)
	 *   Copy 3-4 bytes from block within 2kB distance
	 *   state = S (copy S literals after this block)
	 *   length = 3 + L
	 * Always followed by exactly one byte : H H H H H H H H
	 *   distance = (H << 3) + D + 1
	 */
	if d.src.idx+1 > d.src.end {
		return ErrInputOverrun
	}
	d.lbIdx = d.dst.idx - ((int(d.src.data[d.src.idx]) << 3) + int((inst>>2)&0x7) + 1)
	d.src.idx++
	d.lbLen = int(inst>>5) + 1
	d.nextState = int(inst & 0x3)
	return nil
}

//go:inline
func (d *decoder) handleM3(inst byte) error {
	/* [M3]
	 * 0 0 1 L L L L L  (32..63)
	 *   Copy of small block within 16kB distance (preferably less than 34B)
	 *   length = 2 + (L ?: 31 + (zero_bytes * 255) + non_zero_byte)
	 * Always followed by exactly one LE16 :  D D D D D D D D : D D D D D D S S
	 *   distance = D + 1
	 *   state = S (copy S literals after this block)
	 */
	d.lbLen = int(inst&0x1f) + 2
	if d.lbLen == 2 {
		offset, err := d.countZeroBytes(31)
		if err != nil {
			return err
		}
		d.lbLen += offset
	}
	if d.src.idx+2 > d.src.end {
		return ErrInputOverrun
	}
	val := int(d.getLe16())
	d.src.idx += 2
	d.lbIdx = d.dst.idx - (val>>2 + 1)
	d.nextState = val & 0x3
	return nil
}

//go:inline
func (d *decoder) handleM4(inst byte) (finished bool, err error) {
	/* [M4]
	 * 0 0 0 1 H L L L  (16..31)
	 *   Copy of a block within 16..48kB distance (preferably less than 10B)
	 *   length = 2 + (L ?: 7 + (zero_bytes * 255) + non_zero_byte)
	 * Always followed by exactly one LE16 :  D D D D D D D D : D D D D D D S S
	 *   distance = 16384 + (H << 14) + D
	 *   state = S (copy S literals after this block)
	 *   End of stream is reached if distance == 16384
	 */
	d.lbLen = int(inst&0x7) + 2
	if d.lbLen == 2 {
		offset, err := d.countZeroBytes(7)
		if err != nil {
			return false, err
		}
		d.lbLen += offset
	}

	if d.src.idx+2 > d.src.end {
		return false, ErrInputOverrun
	}

	val := int(d.getLe16())
	d.src.idx += 2
	d.lbIdx = d.dst.idx - (int(inst&0x8)<<11 + val>>2)
	d.nextState = val & 0x3

	if d.lbIdx == d.dst.idx {
		finished = true /* stream finished */
		return finished, nil
	}

	d.lbIdx -= 16384

	return false, nil
}

//go:inline
func (d *decoder) handleM1LongLiteral(inst byte) error {
	/* If last instruction did not copy any literal (state == 0), this
	 * encoding will be a copy of 4 or more literal, and must be interpreted
	 * like this :
	 *
	 *    0 0 0 0 L L L L  (0..15)  : copy long literal string
	 *    length = 3 + (L ?: 15 + (zero_bytes * 255) + non_zero_byte)
	 *    state = 4  (no extra literals are copied)
	 */
	length := int(inst) + 3
	if length == 3 {
		offset, err := d.countZeroBytes(15)
		if err != nil {
			return err
		}

		length += offset
	}

	if d.src.idx+length > d.src.end {
		return ErrInputOverrun
	}

	if d.dst.idx+length > d.dst.end {
		return ErrOutputOverrun
	}

	d.copyLiterals(length)

	return nil
}

//go:inline
func (d *decoder) handleM1ShortCopy(inst byte) error {
	switch {
	case d.curState != 4:
		/* If last instruction used to copy between 1 to 3 literals (encoded in
		 * the instruction's opcode or distance), the instruction is a copy of a
		 * 2-byte block from the dictionary within a 1kB distance. It is worth
		 * noting that this instruction provides little savings since it uses 2
		 * bytes to encode a copy of 2 other bytes but it encodes the number of
		 * following literals for free. It must be interpreted like this :
		 *
		 *    0 0 0 0 D D S S  (0..15)  : copy 2 bytes from <= 1kB distance
		 *    length = 2
		 *    state = S (copy S literals after this block)
		 *  Always followed by exactly one byte : H H H H H H H H
		 *    distance = (H << 2) + D + 1
		 */

		if d.src.idx+1 > d.src.end {
			return ErrInputOverrun
		}

		d.nextState = int(inst & 0x3)
		d.lbIdx = d.dst.idx - (int(inst>>2) + (int(d.src.data[d.src.idx]) << 2) + 1)
		d.src.idx++
		d.lbLen = 2

	default:
		/* If last instruction used to copy 4 or more literals (as detected by
		 * state == 4), the instruction becomes a copy of a 3-byte block from the
		 * dictionary from a 2..3kB distance, and must be interpreted like this :
		 *
		 *    0 0 0 0 D D S S  (0..15)  : copy 3 bytes from 2..3 kB distance
		 *    length = 3
		 *    state = S (copy S literals after this block)
		 *  Always followed by exactly one byte : H H H H H H H H
		 *    distance = (H << 2) + D + 2049
		 */

		if d.src.idx+1 > d.src.end {
			return ErrInputOverrun
		}

		d.nextState = int(inst & 0x3)
		d.lbIdx = d.dst.idx - (int(inst>>2) + (int(d.src.data[d.src.idx]) << 2) + 2049)
		d.src.idx++
		d.lbLen = 3
	}
	return nil
}

//go:inline
func (d *decoder) copyLiterals(length int) {
	// note: benchmarking shows that this is faster than using copy()
	for i := range length {
		d.dst.data[d.dst.idx+i] = d.src.data[d.src.idx+i]
	}
	d.dst.idx += length
	d.src.idx += length
}

//go:inline
func (d *decoder) copyLookbehind() {
	// note: benchmarking shows that this is faster than using copy()
	for i := 0; i < d.lbLen; i++ {
		d.dst.data[d.dst.idx+i] = d.dst.data[d.lbIdx+i]
	}
	d.dst.idx += d.lbLen
	d.curState = d.nextState
}

//go:inline
func (d *decoder) countZeroBytes(factor int) (int, error) {
	old := d.src.idx
	// count how many zero bytes we have until we hit a non-zero byte
	for d.src.idx < d.src.end && d.src.data[d.src.idx] == 0 {
		d.src.idx++
	}

	count := d.src.idx - old

	if count > int(max255Count) {
		return -1, ErrDecompressionFailed
	}
	if d.src.idx+1 > d.src.end {
		return -1, ErrInputOverrun
	}
	offset := count*255 + factor + int(d.src.data[d.src.idx])
	d.src.idx++

	return offset, nil
}

//go:inline
func (d *decoder) getLe16() uint16 {
	if hostBigEndian {
		// swap bytes for big endian
		return uint16(d.src.data[d.src.idx]) | (uint16(d.src.data[d.src.idx+1]) << 8)
	}
	return binary.LittleEndian.Uint16(d.src.data[d.src.idx : d.src.idx+2])
}
