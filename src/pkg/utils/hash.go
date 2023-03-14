// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions
package utils

import (
	"crypto"
	"encoding/hex"
	"hash/crc32"
	"io"
	"os"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// GetCryptoHash returns the computed SHA256 Sum of a given file
func GetCryptoHash(path string, hashName crypto.Hash) (string, error) {
	var data io.ReadCloser
	var err error

	if IsURL(path) {
		// Handle download from URL
		message.Warn("This is a remote source. If a published checksum is available you should use that rather than calculating it directly from the remote link.")
		data = Fetch(path)
	} else {
		// Handle local file
		data, err = os.Open(path)
		if err != nil {
			return "", err
		}
	}

	defer data.Close()

	hash := hashName.New()
	if _, err = io.Copy(hash, data); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetCRCHash returns the computed CRC32 Sum of a given string
func GetCRCHash(text string) uint32 {
	table := crc32.MakeTable(crc32.IEEE)
	return crc32.Checksum([]byte(text), table)
}
