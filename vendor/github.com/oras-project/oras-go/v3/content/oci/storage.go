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
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/ioutil"
)

// bufPool is a pool of byte buffers that can be reused for copying content
// between files.
var bufPool = sync.Pool{
	New: func() any {
		// the buffer size should be larger than or equal to 128 KiB
		// for performance considerations.
		// we choose 1 MiB here so there will be less disk I/O.
		buffer := make([]byte, 1<<20) // buffer size = 1 MiB
		return &buffer
	},
}

// Storage is a CAS based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md
type Storage struct {
	*ReadOnlyStorage

	// MaxUnsafePushBytes is the maximum number of bytes UnsafePush accepts for
	// both known and unknown sizes. If <= 0, the default is used. Set it before
	// using the storage concurrently.
	//   - Default value: 4 GiB.
	MaxUnsafePushBytes int64

	// root is the root directory of the OCI layout.
	root string
	// ingestRoot is the root directory of the temporary ingest files.
	ingestRoot string
}

// ingestDirName is the name of the directory, in the root of the layout, that
// holds the temporary files of blob writes in progress.
const ingestDirName = "ingest"

// NewStorage creates a new CAS based on file system with the OCI-Image layout.
func NewStorage(root string) (*Storage, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", root, err)
	}

	return &Storage{
		ReadOnlyStorage: NewStorageFromFS(os.DirFS(rootAbs)),
		root:            rootAbs,
		ingestRoot:      filepath.Join(rootAbs, ingestDirName),
	}, nil
}

// Push pushes the content, matching the expected descriptor.
func (s *Storage) Push(_ context.Context, expected ocispec.Descriptor, content io.Reader) error {
	path, err := blobPath(expected.Digest)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrInvalidDigest)
	}
	target := filepath.Join(s.root, path)

	// check if the target content already exists in the blob directory.
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := ensureDir(filepath.Dir(target)); err != nil {
		return err
	}

	// write the content to a temporary ingest file.
	ingest, err := s.ingest(expected, content)
	if err != nil {
		return err
	}

	// move the content from the temporary ingest file to the target path.
	// since blobs are read-only once stored, if the target blob already exists,
	// Rename() will fail for permission denied when trying to overwrite it.
	if err := os.Rename(ingest, target); err != nil {
		// remove the ingest file in case of error
		os.Remove(ingest)
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
		}

		return err
	}

	return nil
}

// UnsafePush pushes content whose digest or size may be unknown, subject to
// MaxUnsafePushBytes. See content.UnsafePusher for verification and error semantics.
func (s *Storage) UnsafePush(ctx context.Context, expected ocispec.Descriptor, r io.Reader) (ocispec.Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return ocispec.Descriptor{}, err
	}
	return s.unsafePush(expected, ioutil.NewContextReader(ctx, r), s.MaxUnsafePushBytes)
}

func (s *Storage) unsafePush(expected ocispec.Descriptor, r io.Reader, limit int64) (ocispec.Descriptor, error) {
	if limit <= 0 {
		limit = ioutil.DefaultMaxUnsafePushBytes
	}
	if err := ioutil.ValidateUnsafePush(expected, limit); err != nil {
		return ocispec.Descriptor{}, err
	}
	if expected.Digest != "" && expected.Size >= 0 {
		path, err := blobPath(expected.Digest)
		if err != nil {
			return ocispec.Descriptor{}, err
		}
		if fi, err := os.Stat(filepath.Join(s.root, path)); err == nil {
			if fi.Size() == expected.Size {
				return expected, fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
			}
		} else if !os.IsNotExist(err) {
			return ocispec.Descriptor{}, err
		}
	}
	if err := ensureDir(s.ingestRoot); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to ensure ingest dir: %w", err)
	}
	prefix := "unsafe"
	if expected.Digest != "" {
		prefix = expected.Digest.Encoded()
	}
	fp, err := os.CreateTemp(s.ingestRoot, prefix+"_*")
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to create ingest file: %w", err)
	}
	renamed := false
	defer func() {
		if !renamed {
			os.Remove(fp.Name())
		}
	}()
	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	desc, err := ioutil.CopyUnsafe(fp, r, *buf, expected, limit)
	closeErr := fp.Close()
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to ingest: %w", err)
	}
	if closeErr != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to close ingest file: %w", closeErr)
	}
	path, err := blobPath(desc.Digest)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	target := filepath.Join(s.root, path)
	if err := ensureDir(filepath.Dir(target)); err != nil {
		return ocispec.Descriptor{}, err
	}
	if _, err := os.Stat(target); err == nil {
		return desc, fmt.Errorf("%s: %s: %w", desc.Digest, desc.MediaType, errdef.ErrAlreadyExists)
	} else if !os.IsNotExist(err) {
		return ocispec.Descriptor{}, err
	}
	if err := os.Chmod(fp.Name(), 0444); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to make readonly: %w", err)
	}
	if err := os.Rename(fp.Name(), target); err != nil {
		if errors.Is(err, os.ErrPermission) {
			if _, statErr := os.Stat(target); statErr == nil {
				return desc, fmt.Errorf("%s: %s: %w", desc.Digest, desc.MediaType, errdef.ErrAlreadyExists)
			}
		}
		return ocispec.Descriptor{}, err
	}
	renamed = true
	return desc, nil
}

// Delete removes the target from the system.
func (s *Storage) Delete(ctx context.Context, target ocispec.Descriptor) error {
	path, err := blobPath(target.Digest)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrInvalidDigest)
	}
	targetPath := filepath.Join(s.root, path)
	err = os.Remove(targetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrNotFound)
		}
		return err
	}
	return nil
}

// ingest write the content into a temporary ingest file.
func (s *Storage) ingest(expected ocispec.Descriptor, content io.Reader) (path string, ingestErr error) {
	if err := ensureDir(s.ingestRoot); err != nil {
		return "", fmt.Errorf("failed to ensure ingest dir: %w", err)
	}

	// create a temp file with the file name format "blobDigest_randomString"
	// in the ingest directory.
	// Go ensures that multiple programs or goroutines calling CreateTemp
	// simultaneously will not choose the same file.
	fp, err := os.CreateTemp(s.ingestRoot, expected.Digest.Encoded()+"_*")
	if err != nil {
		return "", fmt.Errorf("failed to create ingest file: %w", err)
	}

	path = fp.Name()
	defer func() {
		// close the temp file and check close error
		if err := fp.Close(); err != nil && ingestErr == nil {
			ingestErr = fmt.Errorf("failed to close ingest file: %w", err)
		}

		// remove the temp file in case of error
		if ingestErr != nil {
			os.Remove(path)
		}
	}()

	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	if err := ioutil.CopyBuffer(fp, content, *buf, expected); err != nil {
		ingestErr = fmt.Errorf("failed to ingest: %w", err)
		return
	}

	// change to readonly
	if err := os.Chmod(path, 0444); err != nil {
		ingestErr = fmt.Errorf("failed to make readonly: %w", err)
		return
	}

	return
}

// ensureDir ensures the directories of the path exists.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0777)
}
