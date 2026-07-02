package nixhash

import (
	"crypto"
	"fmt"
)

// Algorithm represent the hashing algorithm used to digest the data.
type Algorithm uint8

const (
	_ = iota

	// All the algorithms that Nix understands.
	MD5    = Algorithm(iota)
	SHA1   = Algorithm(iota)
	SHA256 = Algorithm(iota)
	SHA512 = Algorithm(iota)
)

func ParseAlgorithm(s string) (Algorithm, error) {
	switch s {
	case "md5":
		return MD5, nil
	case "sha1":
		return SHA1, nil
	case "sha256":
		return SHA256, nil
	case "sha512":
		return SHA512, nil
	default:
		return 0, fmt.Errorf("unknown algorithm: %s", s)
	}
}

func (a Algorithm) String() string {
	switch a {
	case MD5:
		return "md5"
	case SHA1:
		return "sha1"
	case SHA256:
		return "sha256"
	case SHA512:
		return "sha512"
	default:
		panic(fmt.Sprintf("bug: unknown algorithm %d", a))
	}
}

// Func returns the cryptographic hash function for the Algorithm (implementing crypto.Hash)
// It panics when encountering an invalid Algorithm, as these can only occur by
// manually filling the struct.
func (a Algorithm) Func() crypto.Hash {
	switch a {
	case MD5:
		return crypto.MD5
	case SHA1:
		return crypto.SHA1
	case SHA256:
		return crypto.SHA256
	case SHA512:
		return crypto.SHA512
	default:
		panic(fmt.Sprintf("Invalid hash type: %v", a))
	}
}
