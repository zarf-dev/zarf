// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

import (
	"github.com/defenseunicorns/zarf/src/types"
)

// SetBaseDirectory sets base directory on package config when given in args
func SetBaseDirectory(args []string, pkgConfig *types.PackagerConfig) {
	if len(args) > 0 {
		pkgConfig.CreateOpts.BaseDir = args[0]
	} else {
		pkgConfig.CreateOpts.BaseDir = "."
	}
}
