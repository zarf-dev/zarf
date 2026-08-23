// Package storepath parses and renders Nix store paths.
package storepath

import (
	"fmt"
	"path"
	"regexp"

	"github.com/nix-community/go-nix/pkg/nixbase32"
)

const (
	StoreDir     = "/nix/store"
	PathHashSize = 20
)

//nolint:gochecknoglobals
var (
	NameRe = regexp.MustCompile(`[a-zA-Z0-9+\-_?=][.a-zA-Z0-9+\-_?=]*`)
	PathRe = regexp.MustCompile(fmt.Sprintf(
		`^%v/([%v]{%d})-(%v)$`,
		regexp.QuoteMeta(StoreDir),
		nixbase32.Alphabet,
		nixbase32.EncodedLen(PathHashSize),
		NameRe,
	))

	// Length of the hash portion of the store path in base32.
	encodedPathHashSize = nixbase32.EncodedLen(PathHashSize)

	// Offset in absolute string to hash.
	hashOffset = len(StoreDir) + 1
	// Offset in relative path string to name.
	nameOffset = encodedPathHashSize + 1
)

// StorePath represents a bare Nix store path, without any paths underneath `/nix/store/…-…`.
type StorePath struct {
	Name   string
	Digest []byte
}

// String returns a Store without StoreDir.
// It starts with a digest (20 bytes), nixbase32-encoded,
// followed by a `-`, and ends with the name.
func (n *StorePath) String() string {
	return nixbase32.EncodeToString(n.Digest) + "-" + n.Name
}

// Absolute returns a StorePath with StoreDir and slash prepended.
// We use forward slashes on all architectures (including Windows), to be
// consistent in hashing contexts.
func (n *StorePath) Absolute() string {
	return path.Join(StoreDir, n.String())
}

// Validate validates a StorePath, verifying it's syntactically valid.
func (n *StorePath) Validate() error {
	return Validate(n.Absolute())
}

// FromString parses a Nix store path without store prefix into a StorePath,
// verifying it's syntactically valid.
// It returns an error if it fails to parse.
func FromString(s string) (*StorePath, error) {
	if err := Validate(path.Join(StoreDir, s)); err != nil {
		return nil, err
	}

	digest, err := nixbase32.DecodeString(s[:nameOffset-1])
	if err != nil {
		return nil, fmt.Errorf("unable to decode hash: %v", err)
	}

	return &StorePath{
		Name:   s[nameOffset:],
		Digest: digest,
	}, nil
}

// FromAbsolutePath parses an absolute Nix Store path including store prefix)
// into a StorePath, verifying it's syntactically valid.
// It returns an error if it fails to parse.
func FromAbsolutePath(s string) (*StorePath, error) {
	if len(s) < hashOffset+nameOffset+1 {
		return nil, fmt.Errorf("unable to parse path: invalid path length %d for path %v", len(s), s)
	}

	return FromString(s[hashOffset:])
}

// Validate validates an absolute Nix Store Path string.
func Validate(s string) error {
	if len(s) < hashOffset+encodedPathHashSize+1 {
		return fmt.Errorf("unable to parse path: invalid path length %d for path %v", len(s), s)
	}

	if s[:len(StoreDir)] != StoreDir {
		return fmt.Errorf("unable to parse path: mismatching store path prefix for path %v", s)
	}

	if err := nixbase32.ValidateString(s[hashOffset : hashOffset+encodedPathHashSize]); err != nil {
		return fmt.Errorf("unable to parse path: error validating path nixbase32 %v: %v", err, s)
	}

	for _, c := range s[nameOffset:] {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			switch c {
			case '-':
				continue
			case '_':
				continue
			case '.':
				continue
			case '+':
				continue
			case '?':
				continue
			case '=':
				continue
			}

			return fmt.Errorf("unable to parse path: invalid character in path: %v", s)
		}
	}

	return nil
}
