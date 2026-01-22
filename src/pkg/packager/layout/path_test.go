// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestResolvePackagePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		setupPath        string
		createFiles      map[string]string
		createDirs       []string
		createSymlinks   map[string]string
		wantErr          bool
		wantManifest     string
		wantBaseDir      string
		skipPathCreation bool
	}{
		{
			name:         "directory path resolves to zarf.yaml",
			setupPath:    "",
			createFiles:  map[string]string{layout.ZarfYAML: "test"},
			wantManifest: layout.ZarfYAML,
			wantBaseDir:  "",
		},
		{
			name:         "direct file path",
			setupPath:    "custom-package.yaml",
			createFiles:  map[string]string{"custom-package.yaml": "test"},
			wantManifest: "custom-package.yaml",
			wantBaseDir:  "",
		},
		{
			name:         "arbitrarily named file in subdirectory",
			setupPath:    "packages/mypackage/my-custom-zarf.yaml",
			createDirs:   []string{"packages/mypackage"},
			createFiles:  map[string]string{"packages/mypackage/my-custom-zarf.yaml": "test"},
			wantManifest: "packages/mypackage/my-custom-zarf.yaml",
			wantBaseDir:  "packages/mypackage",
		},
		{
			name:         "empty directory without zarf.yaml",
			setupPath:    "",
			wantManifest: layout.ZarfYAML,
			wantBaseDir:  "",
		},
		{
			name:             "non-existent path returns error",
			setupPath:        "does-not-exist",
			skipPathCreation: true,
			wantErr:          true,
		},
		{
			name:         "nested directory path",
			setupPath:    "a/b/c",
			createDirs:   []string{"a/b/c"},
			wantManifest: "a/b/c/" + layout.ZarfYAML,
			wantBaseDir:  "a/b/c",
		},
		{
			name:           "symlink to directory",
			setupPath:      "link",
			createDirs:     []string{"target"},
			createFiles:    map[string]string{"target/" + layout.ZarfYAML: "test"},
			createSymlinks: map[string]string{"link": "target"},
			wantManifest:   "link/" + layout.ZarfYAML,
			wantBaseDir:    "link",
		},
		{
			name:           "symlink to file",
			setupPath:      "link.yaml",
			createFiles:    map[string]string{"target.yaml": "test"},
			createSymlinks: map[string]string{"link.yaml": "target.yaml"},
			wantManifest:   "link.yaml",
			wantBaseDir:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpdir := t.TempDir()

			// Create directories
			for _, dir := range tt.createDirs {
				err := os.MkdirAll(filepath.Join(tmpdir, dir), 0700)
				require.NoError(t, err)
			}

			// Create files
			for file, content := range tt.createFiles {
				err := os.WriteFile(filepath.Join(tmpdir, file), []byte(content), 0600)
				require.NoError(t, err)
			}

			// Create symlinks
			for link, target := range tt.createSymlinks {
				err := os.Symlink(filepath.Join(tmpdir, target), filepath.Join(tmpdir, link))
				require.NoError(t, err)
			}

			// Determine the path to test
			testPath := tmpdir
			if tt.setupPath != "" {
				testPath = filepath.Join(tmpdir, tt.setupPath)
			}

			// Execute
			result, err := layout.ResolvePackagePath(testPath)

			// Validate
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unable to access path")
				require.Equal(t, layout.PackagePath{}, result)
				return
			}

			require.NoError(t, err)

			expectedManifest := filepath.Join(tmpdir, tt.wantManifest)
			require.Equal(t, expectedManifest, result.ManifestFile)

			expectedBaseDir := tmpdir
			if tt.wantBaseDir != "" {
				expectedBaseDir = filepath.Join(tmpdir, tt.wantBaseDir)
			}
			require.Equal(t, expectedBaseDir, result.BaseDir)
		})
	}
}
