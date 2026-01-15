// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"fmt"
	"os"
	"path/filepath"
)

// PackagePath represents a resolved package definition path
type PackagePath struct {
	ManifestFile string // The manifest file (zarf.yaml or arbitrarily named)
	BaseDir      string // Directory for resolving relative paths
}

// ResolvePackagePath takes a user-provided path and resolves it to config file + base dir.
func ResolvePackagePath(path string) (PackagePath, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return PackagePath{}, fmt.Errorf("unable to access path %q: %w", path, err)
	}

	if fileInfo.IsDir() {
		// Backward compatible: directory -> zarf.yaml
		return PackagePath{
			ManifestFile: filepath.Join(path, ZarfYAML),
			BaseDir:      path,
		}, nil
	}

	// Direct file path
	return PackagePath{
		ManifestFile: path,
		BaseDir:      filepath.Dir(path),
	}, nil
}
