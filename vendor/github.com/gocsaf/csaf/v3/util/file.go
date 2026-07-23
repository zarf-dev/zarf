// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2022 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2022 Intevation GmbH <https://intevation.de>

package util

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var invalidRune = regexp.MustCompile(`[^+\-a-z0-9]+`) // invalid runes + `_`

// CleanFileName replaces invalid runes with an underscore and
// afterwards collapses multiple underscores into one.
// If the filename does not end with '.json' it will be appended.
// The filename is converted to lower case.
// https://docs.oasis-open.org/csaf/csaf/v2.0/csd02/csaf-v2.0-csd02.html#51-filename
// specifies valid runes as 'a' to 'z', '0' to '9' and '+', '-', '_'.
func CleanFileName(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSuffix(s, ".json")
	return invalidRune.ReplaceAllString(s, "_") + ".json"
}

// ConformingFileName checks if the given filename conforms to the standard.
func ConformingFileName(fname string) bool {
	return fname == CleanFileName(fname)
}

// IDMatchesFilename checks that filename can be derived from the value
// of document/tracking/id extracted from doc using eval.
// https://docs.oasis-open.org/csaf/csaf/v2.0/os/csaf-v2.0-os.html#51-filename
func IDMatchesFilename(eval *PathEval, doc any, filename string) error {
	var id string
	if err := eval.Extract(`$.document.tracking.id`, StringMatcher(&id), false, doc); err != nil {
		return fmt.Errorf("check that ID matches filename: %v", err)
	}

	if CleanFileName(id) != filename {
		return fmt.Errorf("filename %s does not match document/tracking/id %q",
			filename, id)
	}

	return nil
}

// PathExists returns true if path exits.
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		err = nil
	}
	return false, err
}

// NWriter is an io.Writer counting the bytes copied through it.
type NWriter struct {
	io.Writer
	N int64
}

// Write implements the Write method of io.Writer.
func (nw *NWriter) Write(p []byte) (int, error) {
	n, err := nw.Writer.Write(p)
	nw.N += int64(n)
	return n, err
}

// WriteToFile saves the content of wt into a file names fname.
func WriteToFile(fname string, wt io.WriterTo) error {
	f, err1 := os.Create(fname)
	if err1 != nil {
		return err1
	}
	_, err1 = wt.WriteTo(f)
	err2 := f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// DeepCopy copy a directory tree src to tree dst. Files are hard linked.
func DeepCopy(dst, src string) error {

	stack := []string{dst, src}

	for len(stack) > 0 {
		src = stack[len(stack)-1]
		dst = stack[len(stack)-2]
		stack = stack[:len(stack)-2]

		if err := func() error {
			dir, err := os.Open(src)
			if err != nil {
				return err
			}
			defer dir.Close()

			// Use Readdir as we need no sorting.
			files, err := dir.Readdir(-1)
			if err != nil {
				return err
			}

			for _, f := range files {
				nsrc := filepath.Join(src, f.Name())
				ndst := filepath.Join(dst, f.Name())
				if f.IsDir() {
					// Create new sub dir
					if err := os.Mkdir(ndst, 0755); err != nil {
						return err
					}
					stack = append(stack, ndst, nsrc)
				} else if f.Mode().IsRegular() {
					// Create hard link.
					if err := os.Link(nsrc, ndst); err != nil {
						return err
					}
				}
			}
			return nil
		}(); err != nil {
			return err
		}
	}

	return nil
}

// MakeUniqFile creates a unique named file with the given prefix
// opened in write only mode.
// In case of name collisions the current date plus a random
// number is appended.
func MakeUniqFile(prefix string) (string, *os.File, error) {
	var file *os.File
	name, err := mkUniq(prefix, func(name string) error {
		var err error
		file, err = os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		return err
	})
	return name, file, err
}

// MakeUniqDir creates a unique named directory with the given prefix.
// In case of name collisions the current date plus a random
// number is appended.
func MakeUniqDir(prefix string) (string, error) {
	return mkUniq(prefix, func(name string) error { return os.Mkdir(name, 0755) })
}

func mkUniq(prefix string, create func(string) error) (string, error) {
	now := time.Now()
	stamp := now.Format("-2006-01-02-150405")
	name := prefix + stamp
	err := create(name)
	if err == nil {
		return name, nil
	}
	if os.IsExist(err) {
		rnd := rand.New(rand.NewSource(now.Unix()))

		for i := 0; i < 10000; i++ {
			nname := name + "-" + strconv.FormatUint(uint64(rnd.Uint32()&0xff_ffff), 16)
			err := create(nname)
			if err == nil {
				return nname, nil
			}
			if os.IsExist(err) {
				continue
			}
			return "", err
		}
		return "", &os.PathError{Op: "mkuniq", Path: name, Err: os.ErrExist}
	}

	return "", err
}
