// Package nixhash provides methods to serialize and deserialize some of the
// hashes used in nix code and .narinfo files.
//
// Nix uses different representation of hashes depending on the context
// and history of the project. This package provides the utilities to handle them.
package nixhash

import (
	"encoding/hex"
	"fmt"

	"github.com/nix-community/go-nix/pkg/nixbase32"
)

type Hash struct {
	algo   Algorithm
	digest []byte
}

func NewHash(algo Algorithm, digest []byte) (*Hash, error) {
	if algo.Func().Size() != len(digest) {
		return nil, fmt.Errorf("algo length doesn't match digest size")
	}

	return &Hash{algo, digest}, nil
}

func MustNewHash(algo Algorithm, digest []byte) *Hash {
	h, err := NewHash(algo, digest)
	if err != nil {
		panic(err)
	}

	return h
}

func (h Hash) Algo() Algorithm {
	return h.algo
}

func (h Hash) Digest() []byte {
	return h.digest
}

// Format converts the hash to a string of the given encoding.
func (h Hash) Format(e Encoding, includeAlgo bool) string {
	var s string
	if e == SRI || includeAlgo {
		s += h.algo.String()
		if e == SRI {
			s += "-"
		} else {
			s += ":"
		}
	}

	switch e {
	case Base16:
		s += hex.EncodeToString(h.digest)
	case NixBase32:
		s += nixbase32.EncodeToString(h.digest)
	case Base64, SRI:
		s += b64.EncodeToString(h.digest)
	default:
		panic(fmt.Sprintf("bug: unknown encoding: %v", e))
	}

	return s
}
