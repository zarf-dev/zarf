// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"regexp"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager"
)

// setBaseDirectory sets the base directory. This is a directory with a zarf.yaml.
func setBaseDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}

func defaultRemoteOptions() packager.RemoteOptions {
	return packager.RemoteOptions{
		PlainHTTP:             config.CommonOptions.PlainHTTP,
		InsecureSkipTLSVerify: config.CommonOptions.InsecureSkipTLSVerify,
	}
}

var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)

func getCachePath(ctx context.Context) (string, error) {
	if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
		logger.From(ctx).Warn("invalid characters in Zarf cache path, using default", "cfg", config.ZarfDefaultCachePath, "default", config.ZarfDefaultCachePath)
		config.CommonOptions.CachePath = config.ZarfDefaultCachePath
	}
	return config.GetAbsCachePath()
}
