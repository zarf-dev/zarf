//go:build !cgo
// +build !cgo

package rpmutils

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

func newZstdReader(r io.Reader) (io.ReadCloser, error) {
	decoder, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return zstdCloser{Decoder: decoder}, nil
}

// wrap Decoder so it implements io.Closer properly
type zstdCloser struct {
	*zstd.Decoder
}

func (d zstdCloser) Close() error {
	d.Decoder.Close()
	return nil
}
