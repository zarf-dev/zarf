package derivation

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"
)

//nolint:gochecknoglobals
var (
	textColon     = []byte("text:")
	sha256Colon   = []byte("sha256:")
	storeDirColon = []byte(storepath.StoreDir + ":")
	dotDrv        = []byte(".drv")
)

// Returns the path of a Derivation struct, or an error.
// The path is calculated like this:
//   - Write the fingerprint of the Derivation to the sha256 hash function.
//     This is: `text:`,
//     all d.InputDerivations and d.InputSources (sorted, separated by a `:`),
//     a `:`,
//     a `sha256:`, followed by the sha256 digest of the ATerm representation (hex-encoded)
//     a `:`,
//     the storeDir, followed by a `:`,
//     the name of a derivation,
//     a `.drv`.
//   - Write the .drv A-Term contents to a hash function
//   - Take the digest, run hash.CompressHash(digest, 20) on it.
//   - Encode it with nixbase32
//   - Construct the full path $storeDir/$nixbase32EncodedCompressedHash-$name.drv
func (d *Derivation) DrvPath() (string, error) {
	// calculate the sha256 digest of the ATerm representation
	h := sha256.New()

	if err := d.WriteDerivation(h); err != nil {
		return "", err
	}

	// store the atermDigest, we'll use it later
	atermDigest := h.Sum(nil)

	// reset the sha256 calculator
	h.Reset()

	h.Write(textColon)

	// Write references (lexicographically ordered)
	{
		references := make([]string, len(d.InputDerivations)+len(d.InputSources))

		n := 0

		for inputDrvPath := range d.InputDerivations {
			references[n] = inputDrvPath
			n++
		}

		for _, inputSrc := range d.InputSources {
			references[n] = inputSrc
			n++
		}

		sort.Strings(references)

		for _, ref := range references {
			h.Write(unsafeBytes(ref))
			h.Write(colon)
		}
	}

	h.Write(sha256Colon)

	{
		encoded := make([]byte, hex.EncodedLen(sha256.Size))
		hex.Encode(encoded, atermDigest)
		h.Write(encoded)
	}

	h.Write(colon)
	h.Write(storeDirColon)

	name := d.Name()
	if name == "" {
		// asserted by Validate
		panic("env 'name' not found")
	}

	h.Write(unsafeBytes(name))
	h.Write(dotDrv)

	atermDigest = h.Sum(nil)

	np := storepath.StorePath{
		Name:   name + ".drv",
		Digest: nixhash.CompressHash(atermDigest, 20),
	}

	return np.Absolute(), nil
}
