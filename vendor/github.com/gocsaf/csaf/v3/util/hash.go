// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package util

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"regexp"
)

var hexRe = regexp.MustCompile(`^([[:xdigit:]]+)`)

// HashFromReader reads a base 16 coded hash sum from a reader.
func HashFromReader(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if m := hexRe.FindStringSubmatch(scanner.Text()); m != nil {
			return hex.DecodeString(m[1])
		}
	}
	return nil, scanner.Err()
}

// HashFromFile reads a base 16 coded hash sum from a file.
func HashFromFile(fname string) ([]byte, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return HashFromReader(f)
}

// WriteHashToFile writes a hash of data to file fname.
func WriteHashToFile(fname, name string, h hash.Hash, data []byte) error {
	if _, err := h.Write(data); err != nil {
		return err
	}

	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "%x %s\n", h.Sum(nil), name)
	return f.Close()
}

// WriteHashSumToFile writes a hash sum to file fname.
func WriteHashSumToFile(fname, name string, sum []byte) error {
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "%x %s\n", sum, name)
	return f.Close()
}
