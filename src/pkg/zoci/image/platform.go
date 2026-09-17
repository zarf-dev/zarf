// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zarf-dev/zarf/src/config"
)

var (
	// ErrLayerCompression is returned when a VolumeCompression is not one of the supported formats.
	ErrLayerCompression = errors.New("invalid compression")
	// ErrPlatformOS is returned when a PlatformOS is not one of the supported operating systems.
	ErrPlatformOS = errors.New("invalid platform operating system")
	// ErrPlatformArch is returned when a PlatformArch is not one of the supported architectures.
	ErrPlatformArch = errors.New("invalid platform operating system architecture")
	// ErrTooManyLayers is returned by AddFile when adding another layer would
	// exceed the Volume's MaxLayers.
	ErrTooManyLayers = errors.New("too many image volume layers")
	// ErrNoManifest is returned by WriteTar when it is called before
	// AddDirectory has packed and tagged a manifest.
	ErrNoManifest = errors.New("no image volume manifest: call AddDirectory first")
)

const (
	// DefaultMaxLayers is the layer cap applied to a Volume unless overridden.
	// It matches the classic Docker/graphdriver layer limit that some
	// container runtimes still enforce.
	DefaultMaxLayers uint8 = 127
	// UnlimitedLayers is the Volume.MaxLayers value that disables the layer
	// cap entirely: AddDirectory never batches files into fewer layers.
	UnlimitedLayers uint8 = 0
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

// ValidateCompression returns ErrLayerCompression if format is not one of the
// supported compression formats.
func ValidateCompression(format VolumeCompression) error {
	switch format {
	case VolumeCompressionGzip, VolumeCompressionZstd, VolumeCompressionUncompressed:
		return nil
	default:
		return fmt.Errorf("%w %q, expected one of %s", ErrLayerCompression, format,
			strings.Join([]string{string(VolumeCompressionGzip), string(VolumeCompressionZstd), string(VolumeCompressionUncompressed)}, ", "))
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

// ValidatePlatformOS returns ErrPlatformOS if os is not one of the supported
// operating systems.
func ValidatePlatformOS(os PlatformOS) error {
	switch os {
	case PlatformOSLinux, PlatformOSWindows:
		return nil
	default:
		return fmt.Errorf("%w %q, expected one of %s", ErrPlatformOS, os,
			strings.Join([]string{string(PlatformOSLinux), string(PlatformOSWindows)}, ", "))
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

// ValidatePlatformArch returns ErrPlatformArch if arch is not one of the
// supported architectures.
func ValidatePlatformArch(arch PlatformArch) error {
	switch arch {
	case PlatformArchAMD64, PlatformArchARM64, PlatformArchRISCV:
		return nil
	default:
		return fmt.Errorf("%w %q, expected one of %s", ErrPlatformArch, arch,
			strings.Join([]string{string(PlatformArchAMD64), string(PlatformArchARM64), string(PlatformArchRISCV)}, ", "))
	}
}
