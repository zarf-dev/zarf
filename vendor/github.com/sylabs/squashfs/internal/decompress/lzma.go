package decompress

import (
	"bytes"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

type Lzma struct{}

func (l Lzma) Decompress(data []byte) ([]byte, error) {
	rdr, err := lzma.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(rdr)
}
