package nixhash

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/nix-community/go-nix/pkg/nixbase32"
)

// Parse the hash from a string representation in the format
// "[<type>:]<base16|base32|base64>" or "<type>-<base64>" (a
// Subresource Integrity hash expression). If the 'optAlgo' argument
// is not present, then the hash algorithm must be specified in the
// string.
func ParseAny(s string, optAlgo *Algorithm) (*HashWithEncoding, error) {
	var (
		isSRI = false
		err   error
	)

	h := &HashWithEncoding{}

	// Look for prefix
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		i = strings.IndexByte(s, '-')
		if i > 0 {
			isSRI = true
		}
	}

	// If has prefix, get the algo
	if i > 0 {
		h.includeAlgo = true

		h.algo, err = ParseAlgorithm(s[:i])
		if err != nil {
			return nil, err
		}

		if optAlgo != nil && h.algo != *optAlgo {
			return nil, fmt.Errorf("algo doesn't match expected algo: %v, %v", h.algo, optAlgo)
		}

		// keep the remainder for the encoding
		s = s[i+1:]
	} else if optAlgo != nil {
		h.algo = *optAlgo
	} else {
		return nil, fmt.Errorf("unable to find separator in %v", s)
	}

	// Decode the string. Because we know the algo, and each encoding has a different size, we
	// can find out which of the encoding was used to represent the hash.
	digestLenBytes := h.algo.Func().Size()

	switch len(s) {
	case hex.EncodedLen(digestLenBytes):
		h.encoding = Base16
		h.digest, err = hex.DecodeString(s)
	case nixbase32.EncodedLen(digestLenBytes):
		h.encoding = NixBase32
		h.digest, err = nixbase32.DecodeString(s)
	case b64.EncodedLen(digestLenBytes):
		h.encoding = Base64
		h.digest, err = b64.DecodeString(s)
	default:
		return h, fmt.Errorf("unknown encoding for %v", s)
	}

	if err != nil {
		return h, err
	}

	// Post-processing for SRI
	if isSRI {
		if h.encoding == Base64 {
			h.encoding = SRI
		} else {
			return h, fmt.Errorf("invalid encoding for SRI: %v", h.encoding)
		}
	}

	return h, nil
}

// ParseNixBase32 returns a new Hash struct, by parsing a hashtype:nixbase32 string, or an error.
func ParseNixBase32(s string) (*Hash, error) {
	h, err := ParseAny(s, nil)
	if err != nil {
		return nil, err
	}

	if h.encoding != NixBase32 {
		return nil, fmt.Errorf("expected NixBase32 encoding but got %v", h.encoding)
	}

	return &h.Hash, nil
}

// MustParseNixBase32 returns a new Hash struct, by parsing a hashtype:nixbase32 string, or panics on error.
func MustParseNixBase32(s string) *Hash {
	h, err := ParseNixBase32(s)
	if err != nil {
		panic(err)
	}

	return h
}
