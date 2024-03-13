// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/mholt/archiver/v3"
)

// ComponentSBOM contains paths for a component's SBOM.
type ComponentSBOM struct {
	Files     []string
	Component *ComponentPaths
}

// SBOMs contains paths for SBOMs.
type SBOMs struct {
	Path string
}

// Unarchive unarchives the package's SBOMs.
func (s *SBOMs) Unarchive() (err error) {
	if s.Path == "" || helpers.InvalidPath(s.Path) {
		return &fs.PathError{
			Op:   "stat",
			Path: s.Path,
			Err:  fs.ErrNotExist,
		}
	}
	if helpers.IsDir(s.Path) {
		return nil
	}
	tb := s.Path
	dir := filepath.Join(filepath.Dir(tb), SBOMDir)
	if err := archiver.Unarchive(tb, dir); err != nil {
		return err
	}
	s.Path = dir
	return os.Remove(tb)
}

// Archive archives the package's SBOMs.
func (s *SBOMs) Archive() (err error) {
	if s.Path == "" || helpers.InvalidPath(s.Path) {
		return &fs.PathError{
			Op:   "stat",
			Path: s.Path,
			Err:  fs.ErrNotExist,
		}
	}
	if !helpers.IsDir(s.Path) {
		return nil
	}
	dir := s.Path
	tb := filepath.Join(filepath.Dir(dir), SBOMTar)

	if err := helpers.CreateReproducibleTarballFromDir(dir, "", tb); err != nil {
		return err
	}
	s.Path = tb
	return os.RemoveAll(dir)
}

// IsDir returns true if the SBOMs are a directory.
func (s SBOMs) IsDir() bool {
	return helpers.IsDir(s.Path)
}

// IsTarball returns true if the SBOMs are a tarball.
func (s SBOMs) IsTarball() bool {
	return !s.IsDir() && filepath.Ext(s.Path) == ".tar"
}
