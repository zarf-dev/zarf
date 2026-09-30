// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package checksum computes and verifies file checksums.
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
func VerifyFile(path string, algorithm api.ChecksumAlgorithm, digest string) error {
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
	found, err := getHashOfFile(path, h)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(found, want) != 1 {
		return fmt.Errorf("expected %s of %s to be %s, found %x", algorithm, path, digest, found)
	}
	return nil
}

// GetSHA256OfFile returns the SHA256 hash of the provided file.
func GetSHA256OfFile(path string) (string, error) {
	sum, err := getHashOfFile(path, sha256.New())
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum), nil
}

// GetSHA256Hash returns the SHA256 hash of data read from the provided reader.
func GetSHA256Hash(data io.Reader) (string, error) {
	sum, err := getHash(data, sha256.New())
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum), nil
}

func getHashOfFile(path string, h hash.Hash) (sum []byte, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return getHash(file, h)
}

func getHash(data io.Reader, h hash.Hash) ([]byte, error) {
	if _, err := io.Copy(h, data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
