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

// GetSHA256OfFile returns the SHA256 hash of the provided file.
func GetSHA256OfFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck
	return GetSHA256Hash(file)
}

// SHAsMatch returns an error if the SHA256 hash of the provided file does not match the expected hash.
func SHAsMatch(path, expected string) error {
	actual, err := GetSHA256OfFile(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("expected sha256 of %s to be %s, found %s", path, expected, actual)
	}
	return nil
}

// GetSHA256Hash returns the SHA256 hash of data read from the provided reader.
func GetSHA256Hash(data io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
