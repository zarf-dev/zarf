// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks provides HTTP handlers for the mutating webhook.
package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveOCIProtocol(t *testing.T) {
	OciUrls := []string{
		"oci://google.com",
		"oci://ghcr.io/some/registry",
		"oci://gcr.io/some/registry/here",
		"oci://docker.io/some/registry/here/or/there",
	}

	transformedOciURLs := []string{
		"google.com",
		"ghcr.io/some/registry",
		"gcr.io/some/registry/here",
		"docker.io/some/registry/here/or/there",
	}

	for i, host := range OciUrls {
		newURL, err := removeOCIProtocol(host)
		require.NoError(t, err)
		// For each host/path swap them and add `npm` for compatibility with Gitea/Gitlab
		require.Equal(t, transformedOciURLs[i], newURL)
	}
}
