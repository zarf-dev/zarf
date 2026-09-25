// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package checksum verifies file checksums.
package checksum

import (
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/zarf-dev/zarf/src/api"
)

// VerifyFile checks a file against the expected digest using the given algorithm.
func VerifyFile(path string, algorithm api.ChecksumAlgorithm, digest string) (err error) {
	var h hash.Hash
	switch algorithm {
	case api.ChecksumSHA256:
		h = sha256.New()
	case api.ChecksumSHA512:
		h = sha512.New()
	default:
		return fmt.Errorf("unsupported checksum algorithm %q", algorithm)
	}
	want, err := hex.DecodeString(digest)
	if err != nil {
		return fmt.Errorf("invalid %s checksum %q: %w", algorithm, digest, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	found := h.Sum(nil)
	if subtle.ConstantTimeCompare(found, want) != 1 {
		return fmt.Errorf("expected %s of %s to be %s, found %x", algorithm, path, digest, found)
	}
	return nil
}
