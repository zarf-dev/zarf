// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package image is tooling for creating a multilayered container usable as an image volume mount
// for both containerd and cri-o
package image

import (
	"fmt"
	"maps"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/content/oci"
)

// Options configures the Volume that New builds. The zero value is valid and
// describes an uncompressed image for linux on the architecture Zarf is
// running on, capped at DefaultMaxLayers, carrying no manifest annotations.
type Options struct {
	// OS is the operating system the image targets. The zero value is
	// PlatformOSLinux.
	OS PlatformOS
	// Arch is the CPU architecture the image targets. The zero value is the
	// architecture Zarf is running on, as config.GetArch reports it.
	Arch PlatformArch
	// Compression selects the tar compression format used for the layers the
	// volume pushes. The zero value is VolumeCompressionUncompressed.
	Compression VolumeCompression
	// MaxLayers caps the number of layers AddDirectory will produce: once
	// there are more files than MaxLayers, files are batched several-per-layer
	// to stay within it. AddFile and AddFiles called directly fail once the cap
	// is reached, since there is no further file to batch with. The zero value
	// is DefaultMaxLayers; set UnlimitedLayers to remove the cap instead.
	MaxLayers uint8
	// UnlimitedLayers removes the layer cap entirely, so AddDirectory emits one
	// layer per file however large the tree is. That maximizes blob reuse
	// between builds of the same tree, at the cost of images most runtimes will
	// refuse to mount. Setting it together with MaxLayers returns
	// ErrLayerLimitConflict rather than silently honoring one of them.
	UnlimitedLayers bool
	// Annotations are set on the manifest AddDirectory packs, per the OCI
	// image-spec annotations rules (opaque string key/value metadata; see
	// https://github.com/opencontainers/image-spec/blob/main/annotations.md,
	// e.g. the ocispec.AnnotationTitle/AnnotationCreated pre-defined keys).
	// AddDirectory sets ocispec.AnnotationCreated itself. New copies the map,
	// so the caller's own map is never read again nor written to.
	Annotations map[string]string
}

// New creates a Volume backed by an OCI store at ociDir, with a fresh temp
// workspace and a config stub for the platform opts names.
//
// New returns ErrPlatformOS, ErrPlatformArch, ErrLayerCompression, or
// ErrLayerLimitConflict if opts cannot be satisfied, and does so before
// creating anything on disk. A Volume that New returns is always fully
// configured: every setting lives in Options, so there is nothing left to set
// on the Volume afterwards.
func New(ociDir string, opts Options) (*Volume, error) {
	if opts.OS == "" {
		opts.OS = PlatformOSLinux
	}
	if opts.Arch == "" {
		opts.Arch = PlatformArch(config.GetArch())
	}
	if opts.Compression == "" {
		opts.Compression = VolumeCompressionUncompressed
	}

	if err := ValidatePlatformOS(opts.OS); err != nil {
		return nil, err
	}
	if err := ValidatePlatformArch(opts.Arch); err != nil {
		return nil, err
	}
	if err := ValidateCompression(opts.Compression); err != nil {
		return nil, err
	}
	maxLayers, err := resolveMaxLayers(opts)
	if err != nil {
		return nil, err
	}

	store, err := oci.New(ociDir)
	if err != nil {
		return nil, err
	}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}

	annotations := map[string]string{}
	maps.Copy(annotations, opts.Annotations)

	return &Volume{
		compression: opts.Compression,
		maxLayers:   maxLayers,
		annotations: annotations,
		store:       store,
		tmp:         tmpDir,
		layers:      []ocispec.Descriptor{},
		config: ocispec.Image{
			Platform: ocispec.Platform{
				OS:           string(opts.OS),
				Architecture: string(opts.Arch),
			},
			Created: &static,
			History: []ocispec.History{},
			RootFS: ocispec.RootFS{
				Type:    "layers",
				DiffIDs: []digest.Digest{},
			},
		},
	}, nil
}

// resolveMaxLayers turns the two layer-cap fields in opts into the single
// internal cap a Volume holds, where noLayerLimit means uncapped.
func resolveMaxLayers(opts Options) (uint8, error) {
	switch {
	case opts.UnlimitedLayers && opts.MaxLayers != 0:
		return 0, fmt.Errorf("%w: MaxLayers is %d and UnlimitedLayers is set; pick one", ErrLayerLimitConflict, opts.MaxLayers)
	case opts.UnlimitedLayers:
		return noLayerLimit, nil
	case opts.MaxLayers == 0:
		return DefaultMaxLayers, nil
	default:
		return opts.MaxLayers, nil
	}
}
