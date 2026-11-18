/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oci

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// tempFileAttempts is the number of names tried when creating a temporary
// file. A name carries the 128 bits of randomness of rand.Text, so a single
// collision is already unlikely; retrying is only a guard against a caller
// that has filled the directory with matching names.
const tempFileAttempts = 10

// tempFileSuffixMinLen is the minimum length of the random part of the name of
// a temporary file. rand.Text returns 26 characters today and is documented to
// possibly return more in a future release, so the recognizer accepts any name
// at least this long: a store written by an older binary must stay
// recognizable to a newer one.
const tempFileSuffixMinLen = 26

// fileWrite and fileSync are the functions used to write and synchronize a
// file, overridable in tests.
var (
	fileWrite = (*os.File).Write
	fileSync  = (*os.File).Sync
)

// createTempFile creates a new temporary file in dir whose name is base joined
// with a random string, in the same manner as the ingest files created by
// Storage.
//
// Unlike os.CreateTemp, which always creates the file with permission 0600,
// the file is created with the permission bits perm, so that the process umask
// is applied to it in exactly the same way as it would be by os.WriteFile.
func createTempFile(dir, base string, perm os.FileMode) (*os.File, error) {
	for range tempFileAttempts {
		name := filepath.Join(dir, base+"_"+rand.Text())
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("failed to create temporary file: %w", err)
		}
	}
	return nil, fmt.Errorf("failed to create temporary file in %s: %w", dir, fs.ErrExist)
}

// writeFileAtomic writes data to the file named by path, by writing it to a
// temporary file in the same directory and renaming that file over path.
//
// Unlike os.WriteFile, which truncates the file before writing it, a failed
// write leaves any existing file untouched: the truncation performed by
// os.WriteFile succeeds even when the file system is full, which leaves the
// file empty, or holding only the part of the content that was written before
// the failure.
//
// If sync is true, the content is flushed to stable storage before the file is
// renamed, and the containing directory is flushed afterwards on a best-effort
// basis. Without synchronization, delayed write failures and crash recovery
// are subject to the file system's writeback behavior.
//
// The file is replaced rather than written through, which differs from
// os.WriteFile in ways that matter only outside the layout of a content store.
// path is replaced even if it is a symbolic link, a hard link, or not writable
// by the caller; the mode bits beyond the permission bits, and the ownership,
// access control lists and extended attributes of the file, are not carried
// over; and on Windows the rename fails while any other process holds the file
// open, where a write in place would have succeeded.
func writeFileAtomic(path string, data []byte, sync bool) (writeErr error) {
	// os.WriteFile applies its permission argument only when it creates the
	// file, leaving the permission of an existing file unchanged. Reproduce
	// both behaviors: a file that is being created is created with 0666, so
	// that the umask is applied to it in the same way, and the permission of
	// an existing file is restored onto the temporary file before the rename.
	var perm os.FileMode
	var replacing bool
	if fi, err := os.Stat(path); err == nil {
		perm, replacing = fi.Mode().Perm(), true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	// the temporary file is created no wider than the file it replaces, so
	// that it is never briefly more permissive than the target, and its
	// permission is then set exactly, since the umask only clears bits.
	createPerm := os.FileMode(0666)
	if replacing {
		createPerm = perm
	}
	tempFile, err := createTempFile(dir, filepath.Base(path), createPerm)
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() {
		// remove the temporary file in case of error
		if writeErr != nil {
			tempFile.Close()
			os.Remove(tempPath)
		}
	}()

	if replacing {
		if err := tempFile.Chmod(perm); err != nil {
			return fmt.Errorf("failed to set permission of temporary file: %w", err)
		}
	}
	if _, err := fileWrite(tempFile, data); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if sync {
		// flush the content before closing the file. With delayed allocation, a
		// write can be accepted against space that is never allocated, and the
		// resulting failure is reported neither by Write nor by Close.
		if err := fileSync(tempFile); err != nil {
			return fmt.Errorf("failed to sync temporary file: %w", err)
		}
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}
	if sync {
		// The rename has already made the content visible, so the durability of
		// the directory entry is all that is left to gain here. Reporting a
		// failure would describe an operation that did take effect.
		syncDir(dir)
	}
	return nil
}

// isTempFileOf reports whether name is the name of a temporary file created by
// createTempFile for the file named base.
//
// The random part of the name is matched against the alphabet and the minimum
// length of rand.Text, so that a file that merely shares the prefix, such as a
// copy of index.json that someone has kept, is not mistaken for one of ours.
func isTempFileOf(name, base string) bool {
	suffix, ok := strings.CutPrefix(name, base+"_")
	if !ok || len(suffix) < tempFileSuffixMinLen {
		return false
	}
	// rand.Text returns the RFC 4648 base32 alphabet without padding.
	return !strings.ContainsFunc(suffix, func(r rune) bool {
		return (r < 'A' || r > 'Z') && (r < '2' || r > '7')
	})
}
