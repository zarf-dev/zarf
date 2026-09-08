// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cobra

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

// gzipCobraDesc, zstdCobraDesc, and uncompressedCobraDesc are the shell
// completion descriptions shown alongside each VolumeCompression value.
const (
	gzipCobraDesc         = "gzip-compressed layers"
	zstdCobraDesc         = "zstd-compressed layers"
	uncompressedCobraDesc = "uncompressed layers"
)

// GetDevIVACobraCompression returns the valid VolumeCompression values as
// "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion.
func GetDevIVACobraCompression() []string {
	return []string{
		fmt.Sprintf("%s\t%s", string(image.VolumeCompressionGzip), gzipCobraDesc),
		fmt.Sprintf("%s\t%s", string(image.VolumeCompressionZstd), zstdCobraDesc),
		fmt.Sprintf("%s\t%s", string(image.VolumeCompressionUncompressed), uncompressedCobraDesc),
	}
}

// unlimitedMaxLayersDesc and defaultMaxLayersDesc are the shell completion
// descriptions shown alongside the suggested MaxLayers values.
const (
	unlimitedMaxLayersDesc = "unlimited (disables the cap)"
	defaultMaxLayersDesc   = "default cap"
)

// GetDevIVACobraMaxLayers returns a short list of sensible MaxLayers values as
// "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion. Unlike
// GetCobraCompression/GetCobraPlatformOS this isn't an exhaustive set of
// valid values - MaxLayers accepts any uint8 - just the ones worth
// suggesting.
func GetDevIVACobraMaxLayers() []string {
	return []string{
		fmt.Sprintf("%d\t%s", image.UnlimiteLayers, unlimitedMaxLayersDesc),
		fmt.Sprintf("%d\t%s", image.DefaultMaxLayers, defaultMaxLayersDesc),
	}
}

// osLinuxCobraDesc and osWindowCobraDesc are the shell completion
// descriptions shown alongside each PlatformOS value.
const (
	osLinuxCobraDesc  = "linux image volume"
	osWindowCobraDesc = "windows image volume"
)

// GetDevIVACobraPlatformOS returns the valid PlatformOS values as
// "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion.
func GetDevIVACobraPlatformOS() []string {
	return []string{
		fmt.Sprintf("%s\t%s", string(image.PlatformOSLinux), osLinuxCobraDesc),
		fmt.Sprintf("%s\t%s", string(image.PlatformOSWindows), osWindowCobraDesc),
	}
}
