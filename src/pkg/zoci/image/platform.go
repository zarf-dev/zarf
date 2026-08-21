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
