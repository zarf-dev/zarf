// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validate provides Zarf package validation functions.
package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

func SkeletonPath(path string) error {
	if strings.HasPrefix(path, "file://") {
		return fmt.Errorf("(%s) file:// paths are not supported in skeleton packages", path)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("(%s) absolute paths are not supported in skeleton packages", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("unable to get absolute path for %s: %s", path, err.Error())
	}
	if utils.InvalidPath(abs) {
		return fmt.Errorf("unable to find path %s", path)
	}
	return nil
}
