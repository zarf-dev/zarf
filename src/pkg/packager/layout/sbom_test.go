// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSBOMResourcePaths(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		kind         SBOMResourceKind
		identifier   string
		resourcePath string
	}{
		{
			name:         "component",
			kind:         SBOMResourceComponent,
			identifier:   "metrics",
			resourcePath: "sboms/files/metrics.json",
		},
		{
			name:         "image",
			kind:         SBOMResourceImage,
			identifier:   "registry.example/app:1.2.3/linux/amd64",
			resourcePath: "sboms/images/cmVnaXN0cnkuZXhhbXBsZS9hcHA6MS4yLjMvbGludXgvYW1kNjQ.json",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var actualPath string
			switch test.kind {
			case SBOMResourceComponent:
				actualPath = ComponentSBOMResourcePath(test.identifier)
			case SBOMResourceImage:
				actualPath = ImageSBOMResourcePath(test.identifier)
			}
			require.Equal(t, test.resourcePath, actualPath)

			kind, identifier, ok := ParseSBOMResourcePath(actualPath)
			require.True(t, ok)
			require.Equal(t, test.kind, kind)
			require.Equal(t, test.identifier, identifier)
		})
	}
}
