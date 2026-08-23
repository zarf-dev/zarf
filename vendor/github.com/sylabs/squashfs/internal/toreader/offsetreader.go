package toreader

import "io"

type OffsetReader struct {
	r   io.ReaderAt
	off int64
}

func NewOffsetReader(r io.ReaderAt, off int64) *OffsetReader {
	return &OffsetReader{
		r:   r,
		off: off,
	}
}

func (r OffsetReader) ReadAt(p []byte, off int64) (n int, e error) {
	return r.r.ReadAt(p, off+r.off)
}
