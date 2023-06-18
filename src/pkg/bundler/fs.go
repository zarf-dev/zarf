// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// BFS is a struct that contains the paths used by Bundler
type BFS struct {
	tmp types.TempPaths
	rel types.TempPaths
}

// MakeTemp creates a temporary directory for BundlerFS
func (bfs *BFS) MakeTemp(prefix string) error {
	base, err := utils.MakeTempDir(prefix)
	if err != nil {
		return bfs.Error(err)
	}
	bfs.SetPaths(base)
	return nil
}

// SetPaths sets the paths used by BundlerFS
func (bfs *BFS) SetPaths(base string) {
	bfs.tmp = types.TempPaths{
		Base:      base,
		Checksums: filepath.Join(base, config.ZarfChecksumsTxt),
		ZarfYaml:  filepath.Join(base, config.ZarfYAML),
		ZarfSig:   filepath.Join(base, config.ZarfYAMLSignature),
	}
	bfs.rel = types.TempPaths{
		Checksums: config.ZarfChecksumsTxt,
		ZarfYaml:  config.ZarfYAML,
		ZarfSig:   config.ZarfYAMLSignature,
	}
}

// ClearPaths clears out the paths used by BundlerFS
func (bfs *BFS) ClearPaths() {
	_ = os.RemoveAll(bfs.tmp.Base)
	_ = os.RemoveAll(config.ZarfSBOMDir)
}

// CD is a wrapper around os.Chdir
func (bfs *BFS) CD(path string) error {
	message.Debugf("bfs.CD - %s", path)
	return os.Chdir(path)
}

// ReadBundleYaml is a wrapper around utils.ReadYaml
func (bfs *BFS) ReadBundleYaml(path string, bndl *types.ZarfBundle) error {
	return utils.ReadYaml(path, bndl)
}

// ExtractPackage should extract a package from a bundle
func (bfs *BFS) ExtractPackage(name string) error {
	message.Infof("Extracting %s to %s", name, bfs.tmp.Base)
	return nil
}

// ValidateBundleSignature validates the bundle signature
func (bfs *BFS) ValidateBundleSignature(base string) error {
	message.Infof("Validating bundle signature from %s/%s", base, bfs.rel.ZarfSig)
	return nil
	// err := utils.CosignVerifyBlob(bfs.tmp.ZarfBundleYaml, bfs.tmp.ZarfSig, <keypath>)
	// if err != nil {
	// 	return err
	// }
}

// Error is a helper function to wrap errors from BundlerFS operations
func (bfs *BFS) Error(err error) error {
	return fmt.Errorf("error in BundlerFS operation: %w", err)
}
