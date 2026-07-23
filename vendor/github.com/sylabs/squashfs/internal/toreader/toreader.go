package toreader

import "io"

type Reader struct {
	r      io.ReaderAt
	offset int64
}

func NewReader(r io.ReaderAt, start int64) *Reader {
	return &Reader{
		r:      r,
		offset: start,
	}
}

func (r *Reader) Read(b []byte) (int, error) {
	n, err := r.r.ReadAt(b, r.offset)
	r.offset += int64(n)
	return n, err
}
