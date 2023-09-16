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
	Base string
	Tar  string
}

func (s *SBOMs) Unarchive() (err error) {
	if utils.InvalidPath(s.Tar) {
		return fs.ErrNotExist
	}

	if err := archiver.Unarchive(s.Tar, s.Base); err != nil {
		return err
	}
	s.Tar = ""
	return os.Remove(s.Tar)
}

func (s *SBOMs) Archive() (err error) {
	if utils.InvalidPath(s.Base) {
		return fs.ErrNotExist
	}

	allSBOMFiles, err := filepath.Glob(filepath.Join(s.Base, "*"))
	if err != nil {
		return err
	}

	if err = archiver.Archive(allSBOMFiles, s.Tar); err != nil {
		return
	}
	s.Base = ""
	return os.RemoveAll(s.Base)
}
