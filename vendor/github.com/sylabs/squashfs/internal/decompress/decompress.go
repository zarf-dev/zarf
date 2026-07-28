package decompress

type Decompressor interface {
	Decompress([]byte) ([]byte, error)
}
