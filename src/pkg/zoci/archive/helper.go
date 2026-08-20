// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package archive

import (
	"fmt"
	"regexp"
	"strings"
)

// ImageRefToTar derives a flat tar filename from an image reference, e.g.
// "localhost:5000/rpm:latest" -> "localhost-5000_rpm-latest.tar".
//
// Any digest suffix (@sha256:...) is dropped rather than encoded, so distinct
// digest-pinned references to the same repo:tag collide on one filename.
func ImageRefToTar(ref string) string {
	if ref == "" {
		return ""
	}
	name := ref
	// Drop the digest suffix: it's a fixed-length hex hash, encoding it
	// into the filename would make the name unwieldy for little benefit.
	if i := strings.Index(name, "@"); i != -1 {
		name = name[:i]
	}
	// "/" and ":" are invalid or awkward in filenames on common platforms,
	// so flatten the reference to a single path segment.
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "-")
	return name + ".tar"
}

var (
	// ErrNotTarBall is returned by ValidateFileEndsWithTar when the given
	// path does not end in ".tar".
	ErrNotTarBall = fmt.Errorf("file does not end with \".tar\"")
	tarRegex      = regexp.MustCompile(`\.tar$`)
)

// ValidateFileEndsWithTar returns ErrNotTarBall if file does not end in
// ".tar".
func ValidateFileEndsWithTar(file string) error {
	if !tarRegex.MatchString(file) {
		return ErrNotTarBall
	}
	return nil
}
