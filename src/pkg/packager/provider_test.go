// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_identifySourceType(t *testing.T) {
	sourceMap := map[string]string{
		"oci://ghcr.io/defenseunicorns/packages/init:1.0.0-amd64":                                         "oci",
		"sget://github.com/defenseunicorns/zarf-hello-world:x86":                                          "sget",
		"sget://defenseunicorns/zarf-hello-world:x86_64":                                                  "sget",
		"https://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst": "https",
		"http://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst":  "http",
		"zarf-init-amd64-v1.0.0.tar.zst":                                                                  "tarball",
		"zarf-package-manifests-amd64-v1.0.0.tar":                                                         "tarball",
		"zarf-package-manifests-amd64-v1.0.0.tar.zst":                                                     "tarball",
		"some-dir/.part000": "partial",
	}
	for source, expected := range sourceMap {
		actual := identifySourceType(source)
		require.Equalf(t, expected, actual, fmt.Sprintf("source: %s", source))
	}
}
