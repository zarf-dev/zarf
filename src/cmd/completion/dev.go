// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package completion

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
)

// Descriptions shown alongside each VolumeCompression value.
const (
	gzipDesc         = "gzip-compressed layers"
	zstdDesc         = "zstd-compressed layers"
	uncompressedDesc = "uncompressed layers"
)

// ImageVolumeCompressions returns the valid VolumeCompression values for
// `zarf dev image-volume-archive --layer-compression`.
func ImageVolumeCompressions() []string {
	return []string{
		fmt.Sprintf("%s\t%s", image.VolumeCompressionGzip, gzipDesc),
		fmt.Sprintf("%s\t%s", image.VolumeCompressionZstd, zstdDesc),
		fmt.Sprintf("%s\t%s", image.VolumeCompressionUncompressed, uncompressedDesc),
	}
}

// Descriptions shown alongside the suggested MaxLayers values.
const (
	unlimitedMaxLayersDesc = "unlimited (disables the cap)"
	defaultMaxLayersDesc   = "default cap"
)

// ImageVolumeMaxLayers returns suggestions for
// `zarf dev image-volume-archive --max-layers`. Unlike the other helpers in
// this package this isn't an exhaustive set of valid values - the flag takes
// any uint8 - just the ones worth suggesting.
func ImageVolumeMaxLayers() []string {
	return []string{
		fmt.Sprintf("%d\t%s", image.UnlimitedLayers, unlimitedMaxLayersDesc),
		fmt.Sprintf("%d\t%s", image.DefaultMaxLayers, defaultMaxLayersDesc),
	}
}

// Descriptions shown alongside each PlatformOS value.
const (
	osLinuxDesc   = "linux image volume"
	osWindowsDesc = "windows image volume"
)

// ImageVolumePlatformOSes returns the valid PlatformOS values for
// `zarf dev image-volume-archive --platform-os`.
func ImageVolumePlatformOSes() []string {
	return []string{
		fmt.Sprintf("%s\t%s", image.PlatformOSLinux, osLinuxDesc),
		fmt.Sprintf("%s\t%s", image.PlatformOSWindows, osWindowsDesc),
	}
}
