// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import "fmt"

var (
	// ErrLayerCompression is returned when an ImageVolumeCompression is not one of the supported formats.
	ErrLayerCompression = fmt.Errorf("invalid compression")
	// ErrPlatformOS is returned when an ImagePlatformOS is not one of the supported operating systems.
	ErrPlatformOS = fmt.Errorf("invalid platform operating system")
	// ErrPlatformArch is returned when an ImagePlatformArch is not one of the supported architectures.
	ErrPlatformArch = fmt.Errorf("invalid platform operating system architecture")
)

// ImageVolumeCompression names the tar compression format used for layers.
type ImageVolumeCompression string

// These are the 3 valid tar formats
const (
	// ImageVolumeCompressionGzip is the gzip compression format.
	ImageVolumeCompressionGzip ImageVolumeCompression = "gzip"
	// ImageVolumeCompressionZstd is the zstd compression format.
	ImageVolumeCompressionZstd ImageVolumeCompression = "zstd"
	// ImageVolumeCompressionUncompressed is the uncompressed compression format.
	ImageVolumeCompressionUncompressed ImageVolumeCompression = "uncompressed"
)

// ValidateCompression checks if the given compression format is valid.
func ValidateCompression(format ImageVolumeCompression) error {
	switch format {
	case ImageVolumeCompressionGzip, ImageVolumeCompressionZstd, ImageVolumeCompressionUncompressed:
		return nil
	default:
		return ErrLayerCompression
	}
}

// ImagePlatformOS names an operating system an image volume can target.
type ImagePlatformOS string

// These are the supported operating systems.
const (
	// ImagePlatformOSLinux is the linux operating system.
	ImagePlatformOSLinux ImagePlatformOS = "linux"
	// ImagePlatformOSWindows is the windows operating system.
	ImagePlatformOSWindows ImagePlatformOS = "windows"
)

// ValidatePlatformOS checks if the given platform operating system format is valid.
//
// format: the ImagePlatformOS to validate.
// error: an error if the operating system format is invalid, otherwise nil.
func ValidatePlatformOS(format ImagePlatformOS) error {
	switch format {
	case ImagePlatformOSLinux, ImagePlatformOSWindows:
		return nil
	default:
		return ErrPlatformOS
	}
}

// ImagePlatformArch names a CPU architecture an image volume can target.
type ImagePlatformArch string

// These are the supported architectures.
const (
	// ImagePlatformArchAMD64 is the amd64 architecture.
	ImagePlatformArchAMD64 ImagePlatformArch = "amd64"
	// ImagePlatformArchARM64 is the arm64 architecture.
	ImagePlatformArchARM64 ImagePlatformArch = "arm64"
)

// ValidatePlatformArch checks if the given platform operating system architecture format is valid.
//
// format: the ImagePlatformArch to validate.
// error: an error if the architecture format is invalid, otherwise nil.
func ValidatePlatformArch(format ImagePlatformArch) error {
	switch format {
	case ImagePlatformArchAMD64, ImagePlatformArchARM64:
		return nil
	default:
		return ErrPlatformArch
	}
}
