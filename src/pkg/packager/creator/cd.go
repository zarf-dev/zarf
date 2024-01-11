// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// cdToBaseDir changes the current working directory to the specified base directory.
func cdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error {
	if err := os.Chdir(createOpts.BaseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", createOpts.BaseDir, err)
	}
	message.Note(fmt.Sprintf("Using build directory %s", createOpts.BaseDir))

	// differentials are relative to the current working directory
	if createOpts.DifferentialData.DifferentialPackagePath != "" {
		createOpts.DifferentialData.DifferentialPackagePath = filepath.Join(cwd, createOpts.DifferentialData.DifferentialPackagePath)
	}
	return nil
}
