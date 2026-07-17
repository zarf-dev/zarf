//go:build cgo
// +build cgo

package rpmutils

import (
	"io"

	"github.com/DataDog/zstd"
)

func newZstdReader(r io.Reader) (io.ReadCloser, error) {
	return zstd.NewReader(r), nil
}
