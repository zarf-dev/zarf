// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

func TransformUnkownTarball(path string) (string, error) {
	if utils.InvalidPath(path) {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if filepath.Ext(path) != "" && config.IsValidFileExtension(path) {
		return path, nil
	}

	format, err := archiver.ByExtension(path)
	if err != nil {
		return "", err
	}

	_, ok := format.(*archiver.Tar)
	if ok {
		tb := fmt.Sprintf("%s.tar", path)
		return tb, os.Rename(path, tb)
	}

	_, ok = format.(*archiver.TarZstd)
	if ok {
		tb := fmt.Sprintf("%s.tar.zst", path)
		return tb, os.Rename(path, tb)
	}

	return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, config.GetValidPackageExtensions())
}
