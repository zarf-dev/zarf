// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/types"
)

// setBaseDirectory returns the path to a package definition (a directory containing zarf.yaml,
// or a path to a definition file). args[0] is used if provided, otherwise ".".
func setBaseDirectory(args []string) (string, error) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	// Built-package artifacts (.tar.zst, .tar, .part000*) are rejected with a redirect to the `zarf package` subcommands
	switch {
	case strings.HasSuffix(path, ".tar.zst"), strings.HasSuffix(path, ".tar"):
		return "", fmt.Errorf("%q is a built Zarf package; use a `zarf package` subcommand (e.g. `zarf package inspect`) instead", path)
	case strings.Contains(path, ".part000"):
		return "", fmt.Errorf("%q is a split Zarf package; use a `zarf package` subcommand instead", path)
	}
	return path, nil
}

func defaultRemoteOptions() types.RemoteOptions {
	return types.RemoteOptions{
		PlainHTTP:             plainHTTP,
		InsecureSkipTLSVerify: insecureSkipTLSVerify,
	}
}

var plainHTTP bool
var insecureSkipTLSVerify bool

var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)

func getCachePath(ctx context.Context) (string, error) {
	if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
		logger.From(ctx).Warn("invalid characters in Zarf cache path, using default", "cfg", config.ZarfDefaultCachePath, "default", config.ZarfDefaultCachePath)
		config.CommonOptions.CachePath = config.ZarfDefaultCachePath
	}
	return config.GetAbsCachePath()
}
