package decompress

import (
	"bytes"
	"compress/zlib"
	"io"
)

type Zlib struct{}

func (z Zlib) Decompress(data []byte) ([]byte, error) {
	rdr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	return io.ReadAll(rdr)
}
