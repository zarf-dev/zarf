// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package completion

import (
	"strconv"
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

	// Same agreement the other helpers check, but image.New is what validates
	// a layer cap rather than a Validate function: every suggested value has
	// to build a Volume, including the 0 that means "unlimited" to the flag
	// and "unset" to image.Options.
	for _, v := range values {
		n, err := strconv.ParseUint(v, 10, 8)
		require.NoError(t, err, "suggested %q should parse as a uint8", v)

		iv, err := image.New(t.TempDir(), ImageVolumeOptions(uint8(n)))
		require.NoError(t, err, "suggested %q", v)
		require.NoError(t, iv.Clean())
	}
}

// TestImageVolumeOptions pins the translation the command relies on: the flag
// treats 0 as "no cap", image.Options treats 0 as "use the default", and
// setting both MaxLayers and UnlimitedLayers is an error, so the mapping has
// to pick exactly one of them.
func TestImageVolumeOptions(t *testing.T) {
	t.Parallel()

	unlimited := ImageVolumeOptions(UnlimitedMaxLayers)
	require.True(t, unlimited.UnlimitedLayers)
	require.Zero(t, unlimited.MaxLayers)

	capped := ImageVolumeOptions(42)
	require.False(t, capped.UnlimitedLayers)
	require.Equal(t, uint8(42), capped.MaxLayers)
}

func TestArchitectures(t *testing.T) {
	t.Parallel()

	values := requireCompletionPairs(t, Architectures())
	require.ElementsMatch(t, []string{"amd64", "arm64"}, values)

	for _, v := range values {
		require.NoError(t, image.ValidatePlatformArch(image.PlatformArch(v)), "suggested %q", v)
	}
}
