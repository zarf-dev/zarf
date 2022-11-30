// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images
package images

import "github.com/defenseunicorns/zarf/src/types"

type ImgConfig struct {
	TarballPath string

	ImgList []string

	RegInfo types.RegistryInfo

	NoChecksum bool

	Insecure bool
}

func New(config *ImgConfig) *ImgConfig {
	return config
}
