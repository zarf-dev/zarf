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

type BundlerFS struct {
	tmp types.TempPaths
	rel types.TempPaths
}

func (bfs *BundlerFS) MakeTemp(prefix string) error {
	base, err := utils.MakeTempDir(prefix)
	if err != nil {
		return bfs.Error(err)
	}
	bfs.SetPaths(base)
	return nil
}

func (bfs *BundlerFS) SetPaths(base string) {
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

func (bfs *BundlerFS) ClearPaths() {
	_ = os.RemoveAll(bfs.tmp.Base)
	_ = os.RemoveAll(config.ZarfSBOMDir)
}

func (bfs *BundlerFS) ExtractPackage(name string) error {
	message.Infof("Extracting %s to %s", name, bfs.tmp.Base)
	return nil
}

func (bfs *BundlerFS) Error(err error) error {
	return fmt.Errorf(ErrBundlerFS, err)
}
