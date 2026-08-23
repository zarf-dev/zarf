package decompress

import (
	"bytes"
	"io"

	"github.com/therootcompany/xz"
)

type Xz struct{}

func (x Xz) Decompress(data []byte) ([]byte, error) {
	rdr, err := xz.NewReader(bytes.NewReader(data), 0)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(rdr)
}
