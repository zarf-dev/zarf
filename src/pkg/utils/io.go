// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
)

const (
	tmpPathPrefix = "zarf-"
)

// MakeTempDir creates a temp directory with the zarf- prefix.
func MakeTempDir(basePath string) (string, error) {
	if basePath != "" {
		if err := helpers.CreateDirectory(basePath, 0700); err != nil {
			return "", err
		}
	}
	tmp, err := os.MkdirTemp(basePath, tmpPathPrefix)
	message.Debug("Using temporary directory:", tmp)
	return tmp, err
}

// VerifyBinary returns true if binary is available.
func VerifyBinary(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// GetFinalExecutablePath returns the absolute path to the current executable, following any symlinks along the way.
func GetFinalExecutablePath() (string, error) {
	message.Debug("utils.GetExecutablePath()")

	binaryPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// In case the binary is symlinked somewhere else, get the final destination
	linkedPath, err := filepath.EvalSymlinks(binaryPath)
	return linkedPath, err
}

// GetFinalExecutableCommand returns the final path to the Zarf executable including and library prefixes and overrides.
func GetFinalExecutableCommand() (string, error) {
	// In case the binary is symlinked somewhere else, get the final destination
	zarfCommand, err := GetFinalExecutablePath()
	if err != nil {
		return zarfCommand, err
	}

	if config.ActionsCommandZarfPrefix != "" {
		zarfCommand = fmt.Sprintf("%s %s", zarfCommand, config.ActionsCommandZarfPrefix)
	}

	// If a library user has chosen to override config to use system Zarf instead, reset the binary path.
	if config.ActionsUseSystemZarf {
		zarfCommand = "zarf"
	}

	return zarfCommand, err
}
