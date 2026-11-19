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

package file

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
)

// tarDirectory walks the directory specified by path, and tar those files with a new
// path prefix.
func tarDirectory(ctx context.Context, root, prefix string, w io.Writer, modTime *time.Time, buf []byte) (err error) {
	tw := tar.NewWriter(w)
	defer func() {
		closeErr := tw.Close()
		if err == nil {
			err = closeErr
		}
	}()

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) (returnErr error) {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Rename path
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name = filepath.Join(prefix, name)
		name = filepath.ToSlash(name)

		// Generate header
		// NOTE: We don't support hard links and treat it as regular files
		var link string
		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		header.Name = name
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""

		if modTime != nil {
			if header.ModTime.After(*modTime) {
				header.ModTime = *modTime
			}
			header.AccessTime = header.ModTime
			header.ChangeTime = header.ModTime
		}

		// Write file
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if mode.IsRegular() {
			fp, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() {
				closeErr := fp.Close()
				if returnErr == nil {
					returnErr = closeErr
				}
			}()

			if _, err := ioutil.CopyWithBuffer(tw, fp, buf); err != nil {
				return fmt.Errorf("failed to copy to %s: %w", path, err)
			}
		}

		return nil
	})
}

// extractTarGzip decompresses the gzip
// and extracts tar file to a directory specified by the `dir` parameter.
func extractTarGzip(dirPath, dirName, gzPath, checksumKey, checksum string, buf []byte, preservePermissions bool) (err error) {
	// A non-empty checksum that cannot be parsed must not silently disable
	// the verification of the tar stream.
	var expectedDigest digest.Digest
	if checksum != "" {
		expectedDigest, err = parseTarDigest(checksumKey, checksum)
		if err != nil {
			return err
		}
	}

	fp, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := fp.Close()
		if err == nil {
			err = closeErr
		}
	}()

	gzr, err := gzip.NewReader(fp)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := gzr.Close()
		if err == nil {
			err = closeErr
		}
	}()

	var r io.Reader = gzr
	var verifier digest.Verifier
	if checksum != "" {
		verifier = expectedDigest.Verifier()
		r = io.TeeReader(r, verifier)
	}
	if err := extractTarDirectory(dirPath, dirName, r, buf, preservePermissions); err != nil {
		return err
	}
	if verifier != nil {
		// the tar reader stops at the end-of-archive marker, so read the
		// padding that may follow it into the digest as well
		if _, err := ioutil.CopyWithBuffer(io.Discard, r, buf); err != nil {
			return err
		}
		if !verifier.Verified() {
			return errors.New("content digest mismatch")
		}
	}
	return nil
}

// extractTarDirectory extracts tar file to a directory specified by the `dir`
// parameter. The file name prefix is ensured to be the string specified by the
// `prefix` parameter and is trimmed.
func extractTarDirectory(dirPath, dirName string, r io.Reader, buf []byte, preservePermissions bool) error {
	tr := tar.NewReader(r)
	var dirModes []dirMode
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return restoreDirModes(dirPath, dirModes)
			}
			return err
		}

		// Name check
		filename := header.Name
		filePathRel, err := resolveRelToBase(dirPath, dirName, filename)
		if err != nil {
			return err
		}
		filePath := filepath.Join(dirPath, filePathRel)

		// resolveRelToBase only performs lexical and per-component Lstat checks,
		// which a chain of previously-extracted symlinks can bypass. Re-verify
		// containment with symlinks fully resolved before mutating the
		// filesystem, matching the check on the pushFile path.
		// (GHSA-m37j-52j7-pjw7)
		if err := checkSymlinkEscape(dirPath, filePath); err != nil {
			return err
		}

		// Create content
		var existed bool // the path was there before this directory entry
		switch header.Typeflag {
		case tar.TypeReg:
			err = writeFile(filePath, tr, header.FileInfo().Mode(), buf)
		case tar.TypeDir:
			_, statErr := os.Lstat(filePath)
			existed = statErr == nil
			err = makeDir(filePath, header.FileInfo().Mode())
		case tar.TypeLink:
			// NOTE: ORAS does not generate hard links when creating tarballs.
			// If a hard link is found in the tarball, it will be extracted.
			// If the target link already exists, os.Link will throw an error.
			// This is a known limitation and will not be addressed.
			err = extractHardLink(dirPath, dirName, filePath, header.Linkname)
		case tar.TypeSymlink:
			var target string
			target, err = ensureLinkPath(dirPath, dirName, filePath, header.Linkname)
			if err != nil {
				return err
			}
			if err = os.Symlink(target, filePath); err != nil {
				if !errors.Is(err, fs.ErrExist) {
					return err
				}
				// link already exists, remove the old one and try again
				if err := os.Remove(filePath); err != nil {
					return err
				}
				err = os.Symlink(target, filePath)
			}
			if err != nil {
				return err
			}
			// os.Chtimes and os.Chmod follow symlinks, so restoring the
			// metadata of a symlink entry would modify its target, which may
			// resolve outside the extraction root (GHSA-jh75-xv39-vhvx).
			// Leave the symlink's own metadata as created.
			continue
		default:
			continue // Non-regular files are skipped
		}
		if err != nil {
			return err
		}

		// Never restore metadata through a symlink (GHSA-jh75-xv39-vhvx).
		fi, err := os.Lstat(filePath)
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}

		// Change access time and modification time if possible (error ignored)
		_ = os.Chtimes(filePath, header.AccessTime, header.ModTime)

		// The entries below a directory that its owner cannot read, write or
		// search would fail to extract. Give the owner access now and set the
		// mode after the last entry. A directory that was already there keeps
		// its mode unless permissions are preserved.
		if header.Typeflag == tar.TypeDir && (preservePermissions || !existed) {
			mode := fi.Mode()
			if preservePermissions {
				mode = os.FileMode(header.Mode)
			}
			if mode&0700 != 0700 {
				if err := os.Chmod(filePath, fi.Mode()|0700); err != nil {
					return err
				}
				dirModes = append(dirModes, dirMode{path: filePath, mode: mode})
				continue
			}
		}

		// Restore full mode bits
		if preservePermissions && (header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeDir) {
			if err := os.Chmod(filePath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}
}

// dirMode is the mode to set on a directory once all entries are extracted.
type dirMode struct {
	path string
	mode os.FileMode
}

// restoreDirModes sets the modes of dirs, the last recorded directory first so
// that a directory is set after the directories inside it. A path that is no
// longer a directory is skipped, so a symlink is never followed.
func restoreDirModes(baseAbs string, dirs []dirMode) error {
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := checkSymlinkEscape(baseAbs, dirs[i].path); err != nil {
			return err
		}
		fi, err := os.Lstat(dirs[i].path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if !fi.IsDir() {
			continue
		}
		if err := os.Chmod(dirs[i].path, dirs[i].mode); err != nil {
			return err
		}
	}
	return nil
}

// extractHardLink creates a hard link at filePath to the file named by
// linkname. Unlike a symlink target, a tar hard link target is a path from the
// archive root, the same convention as entry names, so it is resolved against
// dirPath and not against the directory of the link.
func extractHardLink(dirPath, dirName, filePath, linkname string) error {
	rel, err := resolveRelToBase(dirPath, dirName, linkname)
	if err != nil {
		return err
	}
	target := filepath.Join(dirPath, rel)
	if err := checkSymlinkEscape(dirPath, target); err != nil {
		return err
	}
	// link(2) may follow a symlink target on some platforms, and the checks
	// above only inspect the ancestors of target, so refuse a symlink target.
	info, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("hard link %q: %w", linkname, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("hard link target %q is a symbolic link", linkname)
	}
	return os.Link(target, filePath)
}

// resolveRelToBase ensures the target path is in the base path,
// returning its relative path to the base path.
// target can be either an absolute path or a relative path.
func resolveRelToBase(baseAbs, baseRel, target string) (string, error) {
	base := baseRel
	if filepath.IsAbs(target) {
		// ensure base and target are consistent
		base = baseAbs
	}
	path, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("%q is outside of %q", target, baseRel)
	}

	// No symbolic link allowed in the relative path
	dir := filepath.Dir(path)
	for dir != "." {
		if info, err := os.Lstat(filepath.Join(baseAbs, dir)); err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("no symbolic link allowed between %q and %q", baseRel, target)
		}
		dir = filepath.Dir(dir)
	}

	return path, nil
}

// ensureLinkPath ensures the target path pointed by the link is in the base
// path. It returns target path if validated.
func ensureLinkPath(baseAbs, baseRel, link, target string) (string, error) {
	// resolve link
	path := target
	if !filepath.IsAbs(target) {
		path = filepath.Join(filepath.Dir(link), target)
	}
	// ensure path is under baseAbs or baseRel
	if _, err := resolveRelToBase(baseAbs, baseRel, path); err != nil {
		return "", err
	}
	return target, nil
}

// removeSymlink removes path if it is a symlink, so that a subsequent write to
// path does not follow it. It is a no-op if path does not exist or is not a
// symlink.
func removeSymlink(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove symlink %s: %w", path, err)
	}
	return nil
}

// checkSymlinkEscape returns ErrPathTraversalDisallowed if resolving symlinks
// in target's ancestor directories causes it to escape base. target may not
// yet exist, so symlinks are resolved on its deepest existing ancestor.
func checkSymlinkEscape(base, target string) error {
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // base doesn't exist yet; no symlinks to follow
		}
		return err
	}
	realTarget, err := realPathForWrite(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realBase, realTarget)
	if err != nil {
		return ErrPathTraversalDisallowed
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return ErrPathTraversalDisallowed
	}
	return nil
}

// realPathForWrite resolves symlinks in the deepest existing ancestor of path
// and returns the resulting absolute path. Non-existent path components are
// appended verbatim, matching the semantics of a file about to be created.
func realPathForWrite(path string) (string, error) {
	dir := filepath.Dir(path)
	suffix := filepath.Base(path)
	for {
		real, err := filepath.EvalSymlinks(dir)
		if err == nil {
			return filepath.Join(real, suffix), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return path, nil // reached filesystem root
		}
		suffix = filepath.Join(filepath.Base(dir), suffix)
		dir = parent
	}
}

// makeDir creates the directory specified by the `path` parameter.
func makeDir(path string, perm os.FileMode) error {
	// os.MkdirAll follows a terminal symlink and returns nil if it resolves to
	// an existing directory, so a directory entry whose path was already
	// created as a symlink by an earlier archive entry would silently reuse
	// that link, and the metadata restored afterwards would be applied to a
	// directory possibly outside the extraction root (GHSA-jh75-xv39-vhvx).
	// Remove any such symlink first so a real directory is created at path,
	// as writeFile does for regular files.
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	return os.MkdirAll(path, perm)
}

// writeFile writes content to the file specified by the `path` parameter.
func writeFile(path string, r io.Reader, perm os.FileMode, buf []byte) (err error) {
	// os.OpenFile follows a terminal symlink, so a regular-file entry whose
	// path was already created as a symlink by an earlier archive entry would
	// be written through that link, landing outside the extraction root
	// (GHSA-m37j-52j7-pjw7). Remove any such symlink first so the content is
	// written to a regular file at path itself.
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	file, err := createNoFollow(path, perm)
	if errors.Is(err, fs.ErrPermission) {
		// A read-only file, such as one written by an earlier extraction of
		// the same directory, cannot be truncated. Replace it, as tar does.
		if os.Remove(path) == nil {
			file, err = createNoFollow(path, perm)
		}
	}
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}()

	_, err = ioutil.CopyWithBuffer(file, r, buf)
	return err
}

// parseTarDigest parses the digest of an uncompressed tar stream, as recorded
// in the specified directory digest annotation. It returns an error wrapping
// [errdef.ErrUnsupported] if the digest algorithm is not available, and an
// error wrapping [errdef.ErrInvalidDigest] if the value is malformed.
func parseTarDigest(key, checksum string) (digest.Digest, error) {
	d, err := digest.Parse(checksum)
	switch {
	case err == nil:
		return d, nil
	case errors.Is(err, digest.ErrDigestUnsupported):
		return "", fmt.Errorf("%s %q: %v: %w", key, checksum, err, errdef.ErrUnsupported)
	default:
		return "", fmt.Errorf("%s %q: %v: %w", key, checksum, err, errdef.ErrInvalidDigest)
	}
}
