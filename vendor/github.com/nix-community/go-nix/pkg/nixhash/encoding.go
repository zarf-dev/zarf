package nixhash

import (
	"encoding/base64"
)

// Encoding is the string representation of the hashed data.
type Encoding uint8

const (
	_ = iota // ignore zero value

	// All the encodings that Nix understands.
	Base16    = Encoding(iota) // Lowercase hexadecimal encoding.
	Base64    = Encoding(iota) // [IETF RFC 4648, section 4](https://datatracker.ietf.org/doc/html/rfc4648#section-4).
	NixBase32 = Encoding(iota) // Nix-specific base-32 encoding.
	SRI       = Encoding(iota) // W3C recommendation [Subresource Intergrity](https://www.w3.org/TR/SRI/)
)

// b64 is the specific base64 encoding that we are using.
//
//nolint:gochecknoglobals
var b64 = base64.StdEncoding
