// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestAnnotationsFromMetadata(t *testing.T) {
	t.Parallel()

	metadata := v1alpha1.ZarfMetadata{
		Name:          "foo",
		Description:   "bar",
		URL:           "https://example.com",
		Authors:       "Zarf",
		Documentation: "documentation",
		Source:        "source",
		Vendor:        "vendor",
	}
	annotations := annotationsFromMetadata(metadata)
	expectedAnnotations := map[string]string{
		"org.opencontainers.image.title":         "foo",
		"org.opencontainers.image.description":   "bar",
		"org.opencontainers.image.url":           "https://example.com",
		"org.opencontainers.image.authors":       "Zarf",
		"org.opencontainers.image.documentation": "documentation",
		"org.opencontainers.image.source":        "source",
		"org.opencontainers.image.vendor":        "vendor",
	}
	require.Equal(t, expectedAnnotations, annotations)
}
