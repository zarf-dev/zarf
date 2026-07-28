package decompress

import (
	"bytes"
	"io"

	"github.com/klauspost/compress/zstd"
)

type Zstd struct{}

func (z Zstd) Decompress(data []byte) ([]byte, error) {
	rdr, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	return io.ReadAll(rdr)
}
