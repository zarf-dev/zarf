// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import "fmt"

var (
	// ErrLayerCompression is returned when a VolumeCompression is not one of the supported formats.
	ErrLayerCompression = fmt.Errorf("invalid compression")
	// ErrPlatformOS is returned when a PlatformOS is not one of the supported operating systems.
	ErrPlatformOS = fmt.Errorf("invalid platform operating system")
	// ErrPlatformArch is returned when a PlatformArch is not one of the supported architectures.
	ErrPlatformArch = fmt.Errorf("invalid platform operating system architecture")
	// ErrTooManyLayers is returned by AddFile when adding another layer would
	// exceed the Volume's MaxLayers.
	ErrTooManyLayers = fmt.Errorf("too many image volume layers")
)

const (
	// DefaultMaxLayers is the layer cap applied to a Volume unless overridden.
	// It matches the classic Docker/graphdriver layer limit that some
	// container runtimes still enforce.
	DefaultMaxLayers uint8 = 127
	// UnlimiteLayers is the Volume.MaxLayers value that disables the layer
	// cap entirely: AddDirectory never batches files into fewer layers.
	UnlimiteLayers uint8 = 0
)

// unlimitedMaxLayersDesc and defaultMaxLayersDesc are the shell completion
// descriptions shown alongside the suggested MaxLayers values.
const (
	unlimitedMaxLayersDesc = "unlimited (disables the cap)"
	defaultMaxLayersDesc   = "default cap"
)

// GetCobraMaxLayers returns a short list of sensible MaxLayers values as
// "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion. Unlike
// GetCobraCompression/GetCobraPlatformOS this isn't an exhaustive set of
// valid values - MaxLayers accepts any uint8 - just the ones worth
// suggesting.
func GetCobraMaxLayers() []string {
	return []string{
		fmt.Sprintf("%d\t%s", UnlimiteLayers, unlimitedMaxLayersDesc),
		fmt.Sprintf("%d\t%s", DefaultMaxLayers, defaultMaxLayersDesc),
	}
}

// VolumeCompression names the tar compression format used for layers.
type VolumeCompression string

// These are the 3 valid tar formats
const (
	// VolumeCompressionGzip is the gzip compression format.
	VolumeCompressionGzip VolumeCompression = "gzip"
	// VolumeCompressionZstd is the zstd compression format.
	VolumeCompressionZstd VolumeCompression = "zstd"
	// VolumeCompressionUncompressed is the uncompressed compression format.
	VolumeCompressionUncompressed VolumeCompression = "uncompressed"
)

// ValidateCompression checks if the given compression format is valid.
func ValidateCompression(format VolumeCompression) error {
	switch format {
	case VolumeCompressionGzip, VolumeCompressionZstd, VolumeCompressionUncompressed:
		return nil
	default:
		return ErrLayerCompression
	}
}

// gzipCobraDesc, zstdCobraDesc, and uncompressedCobraDesc are the shell
// completion descriptions shown alongside each VolumeCompression value.
const (
	gzipCobraDesc         = "gzip-compressed layers"
	zstdCobraDesc         = "zstd-compressed layers"
	uncompressedCobraDesc = "uncompressed layers"
)

// GetCobraCompression returns the valid VolumeCompression values as
// "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion.
func GetCobraCompression() []string {
	return []string{
		fmt.Sprintf("%s\t%s", string(VolumeCompressionGzip), gzipCobraDesc),
		fmt.Sprintf("%s\t%s", string(VolumeCompressionZstd), zstdCobraDesc),
		fmt.Sprintf("%s\t%s", string(VolumeCompressionUncompressed), uncompressedCobraDesc),
	}
}

// PlatformOS names an operating system an image volume can target.
type PlatformOS string

// These are the supported operating systems.
const (
	// PlatformOSLinux is the linux operating system.
	PlatformOSLinux PlatformOS = "linux"
	// PlatformOSWindows is the windows operating system.
	PlatformOSWindows PlatformOS = "windows"
)

// ValidatePlatformOS checks if the given platform operating system format is valid.
//
// format: the PlatformOS to validate.
// error: an error if the operating system format is invalid, otherwise nil.
func ValidatePlatformOS(format PlatformOS) error {
	switch format {
	case PlatformOSLinux, PlatformOSWindows:
		return nil
	default:
		return ErrPlatformOS
	}
}

// osLinuxCobraDesc and osWindowCobraDesc are the shell completion
// descriptions shown alongside each PlatformOS value.
const (
	osLinuxCobraDesc  = "linux image volume"
	osWindowCobraDesc = "windows image volume"
)

// GetCobraPlatformOS returns the valid PlatformOS values as
// "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion.
func GetCobraPlatformOS() []string {
	return []string{
		fmt.Sprintf("%s\t%s", string(PlatformOSLinux), osLinuxCobraDesc),
		fmt.Sprintf("%s\t%s", string(PlatformOSWindows), osWindowCobraDesc),
	}
}

// PlatformArch names a CPU architecture an image volume can target.
type PlatformArch string

// These are the supported architectures.
const (
	// PlatformArchAMD64 is the amd64 architecture.
	PlatformArchAMD64 PlatformArch = "amd64"
	// PlatformArchARM64 is the arm64 architecture.
	PlatformArchARM64 PlatformArch = "arm64"
)

// ValidatePlatformArch checks if the given platform operating system architecture format is valid.
//
// format: the PlatformArch to validate.
// error: an error if the architecture format is invalid, otherwise nil.
func ValidatePlatformArch(format PlatformArch) error {
	switch format {
	case PlatformArchAMD64, PlatformArchARM64:
		return nil
	default:
		return ErrPlatformArch
	}
}
