// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

type ComponentSBOM struct {
	Files     []string
	Component *ComponentPaths
}

type SBOMs struct {
	Path string
}

func (s *SBOMs) Unarchive() (err error) {
	if utils.InvalidPath(s.Path) {
		return fs.ErrNotExist
	}
	if utils.IsDir(s.Path) {
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

func (s *SBOMs) Archive() (err error) {
	if utils.InvalidPath(s.Path) {
		return fs.ErrNotExist
	}
	if !utils.IsDir(s.Path) {
		return nil
	}
	dir := s.Path
	tb := filepath.Join(filepath.Dir(dir), SBOMTar)

	allSBOMFiles, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}

	if err = archiver.Archive(allSBOMFiles, tb); err != nil {
		return
	}
	s.Path = tb
	return os.RemoveAll(dir)
}
