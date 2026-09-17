// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package completion

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

// requireCompletionPairs checks the shape cobra expects: one
// "value\tdescription" pair per line, both halves non-empty.
func requireCompletionPairs(t *testing.T, got []string) []string {
	t.Helper()

	values := make([]string, 0, len(got))
	for _, line := range got {
		value, desc, found := strings.Cut(line, "\t")
		require.True(t, found, "%q should be a value/description pair separated by a tab", line)
		require.NotEmpty(t, value)
		require.NotEmpty(t, desc)
		values = append(values, value)
	}
	return values
}

func TestImageVolumeCompressions(t *testing.T) {
	t.Parallel()

	values := requireCompletionPairs(t, ImageVolumeCompressions())
	require.ElementsMatch(t, []string{"gzip", "zstd", "uncompressed"}, values)

	// The suggestions and the validator have to agree, or tab completion
	// hands the user a value the command then rejects.
	for _, v := range values {
		require.NoError(t, image.ValidateCompression(image.VolumeCompression(v)), "suggested %q", v)
	}
}

func TestImageVolumePlatformOSes(t *testing.T) {
	t.Parallel()

	values := requireCompletionPairs(t, ImageVolumePlatformOSes())
	require.ElementsMatch(t, []string{"linux", "windows"}, values)

	for _, v := range values {
		require.NoError(t, image.ValidatePlatformOS(image.PlatformOS(v)), "suggested %q", v)
	}
}

func TestImageVolumeMaxLayers(t *testing.T) {
	t.Parallel()

	values := requireCompletionPairs(t, ImageVolumeMaxLayers())
	require.Equal(t, []string{"0", "127"}, values)
}

func TestArchitectures(t *testing.T) {
	t.Parallel()

	values := requireCompletionPairs(t, Architectures())
	require.ElementsMatch(t, []string{"amd64", "arm64"}, values)

	for _, v := range values {
		require.NoError(t, image.ValidatePlatformArch(image.PlatformArch(v)), "suggested %q", v)
	}
}
