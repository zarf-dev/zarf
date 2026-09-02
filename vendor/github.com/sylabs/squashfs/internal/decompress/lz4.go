package decompress

import (
	"bytes"
	"io"

	"github.com/pierrec/lz4/v4"
)

type Lz4 struct{}

func (l Lz4) Decompress(data []byte) ([]byte, error) {
	rdr := lz4.NewReader(bytes.NewReader(data))
	return io.ReadAll(rdr)
}
