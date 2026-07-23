// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package getter

import (
	"io"
	"os"
	"path/filepath"
)

// mkdirTemp creates a new temporary directory that isn't yet created. This
// can be used with calls that expect a non-existent directory.
//
// The temporary directory is also evaluated for symlinks upon creation
// as some operating systems provide symlinks by default when created.
//
// The directory is created as a child of a temporary directory created
// within the directory dir starting with prefix. The temporary directory
// returned is always named "temp". The parent directory has the specified
// prefix.
//
// The returned io.Closer should be used to clean up the returned directory.
// This will properly remove the returned directory and any other temporary
// files created.
//
// If an error is returned, the Closer does not need to be called (and will
// be nil).
func mkdirTemp(dir, prefix string) (string, io.Closer, error) {
	// Create the temporary directory
	td, err := os.MkdirTemp(dir, prefix)
	if err != nil {
		return "", nil, err
	}

	// we evaluate symlinks as some operating systems (eg: MacOS), that
	// actually has any temporary directory created as a symlink.
	// As we have only just created the temporary directory, this is a safe
	// evaluation to make at this time.
	td, err = filepath.EvalSymlinks(td)
	if err != nil {
		return "", nil, err
	}

	return filepath.Join(td, "temp"), pathCloser(td), nil
}

// pathCloser implements io.Closer to remove the given path on Close.
type pathCloser string

// Close deletes this path.
func (p pathCloser) Close() error {
	return os.RemoveAll(string(p))
}
