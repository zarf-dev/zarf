package parser

import (
	"encoding/binary"
	"io"
)

func WithOffset(offset int64) offsetReader {
	return func(io.ReaderAt) (int64, error) {
		return offset, nil
	}
}

func WithIndirectOffsetShortLittleEndian(offset int64) offsetReader {
	return func(r io.ReaderAt) (int64, error) {
		b := make([]byte, 2)
		n, err := r.ReadAt(b, offset)
		if err != nil {
			return 0, err
		}
		if n != len(b) {
			return 0, nil
		}
		return int64(binary.LittleEndian.Uint16(b)), nil
	}
}

func WithChainedOffsetReaders(or ...offsetReader) offsetReader {
	return func(r io.ReaderAt) (int64, error) {
		var offset int64
		for _, o := range or {
			offsetNext, err := o(r)
			if err != nil {
				return 0, err
			}
			offset += offsetNext
		}
		return offset, nil
	}
}
