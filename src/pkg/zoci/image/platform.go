// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/config"
)

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

// PlatformOS names an operating system an image volume can target.
type PlatformOS string

// These are the supported operating systems.
const (
	// PlatformOSLinux is the linux operating system.
	PlatformOSLinux PlatformOS = config.OSLinux
	// PlatformOSWindows is the windows operating system.
	PlatformOSWindows PlatformOS = config.OSWindows
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

// PlatformArch names a CPU architecture an image volume can target.
type PlatformArch string

// These are the supported architectures.
const (
	// PlatformArchAMD64 is the amd64 architecture.
	PlatformArchAMD64 PlatformArch = config.OSArchAMD64
	// PlatformArchARM64 is the arm64 architecture.
	PlatformArchARM64 PlatformArch = config.OSArchARM64
	// PlatformArchRISCV is the riscv architecture.
	PlatformArchRISCV PlatformArch = config.OSArchRISCV
)

// ValidatePlatformArch checks if the given platform operating system architecture format is valid.
//
// format: the PlatformArch to validate.
// error: an error if the architecture format is invalid, otherwise nil.
func ValidatePlatformArch(format PlatformArch) error {
	switch format {
	case PlatformArchAMD64, PlatformArchARM64, PlatformArchRISCV:
		return nil
	default:
		return ErrPlatformArch
	}
}
