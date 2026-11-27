// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// GetImageDigests returns a map of image references to the digests stored in the package's images/index.json.
// References are keyed by the base image name annotation that Zarf writes at create time.
// An empty map is returned when the package does not contain an image index.
func (p *PackageLayout) GetImageDigests() (map[string]string, error) {
	return getImageDigests(filepath.Join(p.dirPath, IndexPath))
}

func getImageDigests(indexPath string) (map[string]string, error) {
	digests := map[string]string{}
	b, err := os.ReadFile(indexPath)
	if errors.Is(err, os.ErrNotExist) {
		return digests, nil
	}
	if err != nil {
		return nil, err
	}
	var index ocispec.Index
	if err := json.Unmarshal(b, &index); err != nil {
		return nil, fmt.Errorf("unable to parse image index %s: %w", indexPath, err)
	}
	for _, manifest := range index.Manifests {
		ref := manifest.Annotations[ocispec.AnnotationBaseImageName]
		if ref == "" {
			continue
		}
		digests[ref] = manifest.Digest.String()
	}
	return digests, nil
}
