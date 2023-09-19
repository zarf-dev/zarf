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

type SBOMs string

func (s SBOMs) Unarchive() (err error) {
	if utils.InvalidPath(string(s)) {
		return fs.ErrNotExist
	}
	if utils.IsDir(string(s)) {
		return nil
	}
	tb := string(s)
	dir := filepath.Join(filepath.Dir(tb), "zarf-sboms")
	if err := archiver.Unarchive(tb, dir); err != nil {
		return err
	}
	s = SBOMs(dir)
	return os.Remove(tb)
}

func (s SBOMs) Archive() (err error) {
	if utils.InvalidPath(string(s)) {
		return fs.ErrNotExist
	}
	if !utils.IsDir(string(s)) {
		return nil
	}
	dir := string(s)
	tb := filepath.Join(filepath.Dir(dir), "sboms.tar")

	allSBOMFiles, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}

	if err = archiver.Archive(allSBOMFiles, tb); err != nil {
		return
	}
	s = SBOMs(tb)
	return os.RemoveAll(dir)
}
