// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package config stores the global configuration and constants.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/types"
)

// Zarf Global Configuration Constants.
const (
	GithubProject = "zarf-dev/zarf"

	ZarfCleanupScriptsPath = "/opt/zarf"

	ZarfPackagePrefix = "zarf-package-"

	ZarfDeployStage = "Deploy"
	ZarfCreateStage = "Create"
	ZarfMirrorStage = "Mirror"
)

// Zarf Constants for In-Cluster Services.
const (
	ZarfImagePullSecretName = "private-registry"
	ZarfGitServerSecretName = "private-git-server"

	UnsetCLIVersion = "unset-development-only"
)

// Zarf Global Configuration Variables.
var (
	// CLIVersion track the version of the CLI
	CLIVersion = UnsetCLIVersion

	// ActionsUseSystemZarf sets whether to use Zarf from the system path if Zarf is being used as a library
	ActionsUseSystemZarf = true

	// ActionsCommandZarfPrefix sets a sub command prefix that Zarf commands are under in the current binary if Zarf is being used as a library (and use system Zarf is not specified)
	ActionsCommandZarfPrefix = ""

	// CommonOptions tracks user-defined values that apply across commands
	CommonOptions types.ZarfCommonOptions

	// Timestamp of when the CLI was started
	operationStartTime  = time.Now().Unix()
	dataInjectionMarker = ".zarf-injection-%d"

	ZarfDefaultCachePath = filepath.Join("~", ".zarf-cache")

	// Default Time Vars
	ZarfDefaultTimeout       = 15 * time.Minute
	ZarfDefaultRetries       = 3
	ZarfDefaultRetryDelay    = 500 * time.Millisecond
	ZarfDefaultRetryMaxDelay = 8 * time.Second
)

// GetStartTime returns the timestamp of when the CLI was started.
func GetStartTime() int64 {
	return operationStartTime
}

// GetDataInjectionMarker returns the data injection marker based on the current CLI start time.
func GetDataInjectionMarker() string {
	return fmt.Sprintf(dataInjectionMarker, operationStartTime)
}

// GetAbsHomePath replaces ~ with the absolute path to a user's home dir
func GetAbsHomePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		homePath, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return strings.Replace(path, "~", homePath, 1), nil
	}
	return path, nil
}

// GetInitPackageName returns the init filename for the architecture (or host) and CLI version.
func GetInitPackageName(architecture string) string {
	arch := helpers.ResolveArch(architecture)
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, CLIVersion)
}
