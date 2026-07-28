package nixhash

// HashWithEncoding stores the original encoding so the user can get error messages with the same encoding.
type HashWithEncoding struct {
	Hash
	encoding    Encoding
	includeAlgo bool
}

func NewHashWithEncoding(
	algo Algorithm,
	digest []byte,
	encoding Encoding,
	includeAlgo bool,
) (*HashWithEncoding, error) {
	h, err := NewHash(algo, digest)
	if err != nil {
		return nil, err
	}

	return &HashWithEncoding{
		Hash:        *h,
		encoding:    encoding,
		includeAlgo: includeAlgo,
	}, nil
}

func MustNewHashWithEncoding(algo Algorithm, digest []byte, encoding Encoding, includeAlgo bool) *HashWithEncoding {
	h := MustNewHash(algo, digest)

	return &HashWithEncoding{
		Hash:        *h,
		encoding:    encoding,
		includeAlgo: includeAlgo,
	}
}

// String return the previous representation of a given hash.
func (h HashWithEncoding) String() string {
	return h.Format(h.encoding, h.includeAlgo)
}
