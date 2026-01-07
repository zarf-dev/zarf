// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// packagePath represents a resolved package definition path
type packagePath struct {
	manifestFile string // The manifest file (zarf.yaml or arbitrarily named)
	baseDir      string // Directory for resolving relative paths
}

// resolvePackagePath takes a user-provided path and resolves it to config file + base dir.
func resolvePackagePath(path string) (packagePath, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return packagePath{}, fmt.Errorf("unable to access path %q: %w", path, err)
	}

	if fileInfo.IsDir() {
		// Backward compatible: directory -> zarf.yaml
		return packagePath{
			manifestFile: filepath.Join(path, layout.ZarfYAML),
			baseDir:      path,
		}, nil
	}

	// Direct file path
	return packagePath{
		manifestFile: path,
		baseDir:      filepath.Dir(path),
	}, nil
}
