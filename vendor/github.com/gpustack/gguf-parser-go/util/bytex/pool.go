package bytex

import (
	"bytes"
	"sync"
)

const defaultSize = 32 * 1024

type (
	Bytes       = []byte
	BytesBuffer = *bytes.Buffer
)

var gp = sync.Pool{
	New: func() any {
		buf := make(Bytes, defaultSize)
		return &buf
	},
}

// GetBytes gets a bytes buffer from the pool,
// which can specify with a size,
// default is 32k.
func GetBytes(size ...uint64) Bytes {
	buf := *(gp.Get().(*Bytes))

	s := defaultSize
	if len(size) != 0 {
		s = int(size[0])
		if s == 0 {
			s = defaultSize
		}
	}
	if cap(buf) >= s {
		return buf[:s]
	}

	gp.Put(&buf)

	ns := s
	if ns < defaultSize {
		ns = defaultSize
	}
	buf = make(Bytes, ns)
	return buf[:s]
}

// WithBytes relies on GetBytes to get a buffer,
// calls the function with the buffer,
// finally, puts it back to the pool after the function returns.
func WithBytes(fn func(Bytes) error, size ...uint64) error {
	if fn == nil {
		return nil
	}

	buf := GetBytes(size...)
	defer Put(buf)
	return fn(buf)
}

// GetBuffer is similar to GetBytes,
// but it returns the bytes buffer wrapped by bytes.Buffer.
func GetBuffer(size ...uint64) BytesBuffer {
	return bytes.NewBuffer(GetBytes(size...)[:0])
}

// WithBuffer relies on GetBuffer to get a buffer,
// calls the function with the buffer,
// finally, puts it back to the pool after the function returns.
func WithBuffer(fn func(BytesBuffer) error, size ...uint64) error {
	if fn == nil {
		return nil
	}

	buf := GetBuffer(size...)
	defer Put(buf)
	return fn(buf)
}

// Put puts the buffer(either Bytes or BytesBuffer) back to the pool.
func Put[T Bytes | BytesBuffer](buf T) {
	switch v := any(buf).(type) {
	case Bytes:
		gp.Put(&v)
	case BytesBuffer:
		bs := v.Bytes()
		gp.Put(&bs)
		v.Reset()
	}
}
