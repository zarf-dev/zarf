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
	"io/fs"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// LayoutFinding is a problem with the structure of an OCI image layout, as
// opposed to the content of a blob.
type LayoutFinding struct {
	// Path is the path of the affected file or directory relative to the root
	// of the layout, using slash separators as in [io/fs].
	Path string
	// Reason describes the problem.
	Reason error
	// ModTime is the modification time of a leftover of an interrupted write,
	// so that a caller can tell a stale file from one that is being written
	// now. It is the zero time for any other finding.
	ModTime time.Time
}

var errInterruptedWrite = errors.New("leftover of an interrupted write")

// verifyLayout checks the structure of the layout in fsys, outside of blobs/,
// and records what it finds in acc. It returns the parsed index.json, or nil if
// that file is missing or invalid. Only a failure to read the layout root, or a
// cancelled context, is returned as an error.
func verifyLayout(ctx context.Context, fsys fs.FS, acc *verifyAccumulator) (*ocispec.Index, error) {
	if layout, err := readOCILayoutFile(fsys); err != nil {
		acc.addLayout(LayoutFinding{Path: ocispec.ImageLayoutFile, Reason: err})
	} else if err := validateOCILayout(layout); err != nil {
		acc.addLayout(LayoutFinding{
			Path:   ocispec.ImageLayoutFile,
			Reason: fmt.Errorf("imageLayoutVersion %q: %w", layout.Version, err),
		})
	}
	index, err := readIndexFile(fsys)
	if err != nil {
		acc.addLayout(LayoutFinding{Path: ocispec.ImageIndexFile, Reason: err})
		index = nil
	}
	if err := isContextDone(ctx); err != nil {
		return nil, err
	}

	rootEntries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read layout root: %w", err)
	}
	// the temporary files of metadata writes sit next to the file they replace.
	for _, entry := range rootEntries {
		name := entry.Name()
		if entry.Type().IsRegular() &&
			(isTempFileOf(name, ocispec.ImageIndexFile) || isTempFileOf(name, ocispec.ImageLayoutFile)) {
			acc.addLayout(leftoverFinding(name, entry))
		}
	}

	// ingest/ only exists in a layout that a Store has written to on disk, so
	// its absence is not a finding.
	ingestEntries, err := fs.ReadDir(fsys, ingestDirName)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			acc.addLayout(LayoutFinding{Path: ingestDirName, Reason: fmt.Errorf("read directory: %w", err)})
		}
		return index, nil
	}
	for _, entry := range ingestEntries {
		if entry.Type().IsRegular() {
			acc.addLayout(leftoverFinding(ingestDirName+"/"+entry.Name(), entry))
		}
	}
	return index, nil
}

func leftoverFinding(p string, entry fs.DirEntry) LayoutFinding {
	finding := LayoutFinding{Path: p, Reason: errInterruptedWrite}
	if fi, err := entry.Info(); err == nil {
		finding.ModTime = fi.ModTime()
	}
	return finding
}
