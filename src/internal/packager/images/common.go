// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/types"
)

// PullConfig is the configuration for pulling images.
type PullConfig struct {
	Concurrency int

	DestinationDirectory string

	ImageList []transform.Image

	Arch string

	RegistryOverrides map[string]string

	CacheDirectory string

	PlainHTTP bool
}

const (
	DockerMediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	DockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

const (
	DockerLayer                    = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	DockerUncompressedLayer        = "application/vnd.docker.image.rootfs.diff.tar"
	OCILayer                       = "application/vnd.oci.image.layer.v1.tar+gzip"
	OCILayerZStd                   = "application/vnd.oci.image.layer.v1.tar+zstd"
	OCIUncompressedLayer           = "application/vnd.oci.image.layer.v1.tar"
	DockerForeignLayer             = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
	OCIRestrictedLayer             = "application/vnd.oci.image.layer.nondistributable.v1.tar+gzip"
	OCIUncompressedRestrictedLayer = "application/vnd.oci.image.layer.nondistributable.v1.tar"
)

func isLayer(mediaType string) bool {
	switch mediaType {
	case DockerLayer, DockerUncompressedLayer, OCILayer, OCILayerZStd, OCIUncompressedLayer, DockerForeignLayer, OCIRestrictedLayer, OCIUncompressedRestrictedLayer:
		return true
	}
	return false
}

func OnlyHasImageLayers(manifest ocispec.Manifest) bool {
	for _, layer := range manifest.Layers {
		if !isLayer(string(layer.MediaType)) {
			return false
		}
	}
	return true
}

func isManifest(mediaType string) bool {
	switch mediaType {
	case ocispec.MediaTypeImageManifest, DockerMediaTypeManifest:
		return true
	}
	return false
}
func isIndex(mediaType string) bool {
	switch mediaType {
	case ocispec.MediaTypeImageIndex, DockerMediaTypeManifestList:
		return true
	}
	return false
}

func getIndexFromOCILayout(dir string) (ocispec.Index, error) {
	idxPath := filepath.Join(dir, "index.json")
	b, err := os.ReadFile(idxPath)
	if err != nil {
		return ocispec.Index{}, fmt.Errorf("failed to get index.json: %w", err)
	}
	var idx ocispec.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return ocispec.Index{}, fmt.Errorf("unable to unmarshal index.json: %w", err)
	}
	return idx, nil
}

func saveIndexToOCILayout(dir string, idx ocispec.Index) error {
	idxPath := filepath.Join(dir, "index.json")
	b, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("unable to marshal index.json: %w", err)
	}
	err = os.WriteFile(idxPath, b, 0o644)
	if err != nil {
		return fmt.Errorf("failed to save changes to index.json: %w", err)
	}
	return nil
}

// PushConfig is the configuration for pushing images.
type PushConfig struct {
	Concurrency int

	SourceDirectory string

	ImageList []transform.Image

	RegInfo types.RegistryInfo

	NoChecksum bool

	Arch string

	Retries int

	PlainHTTP bool
}

// NoopOpt is a no-op option for crane.
func NoopOpt(*crane.Options) {}

// WithGlobalInsecureFlag returns an option for crane that configures insecure
// based upon Zarf's global --insecure-skip-tls-verify (and --insecure) flags.
func WithGlobalInsecureFlag() []crane.Option {
	if config.CommonOptions.InsecureSkipTLSVerify {
		return []crane.Option{crane.Insecure}
	}
	// passing a nil option will cause panic
	return []crane.Option{NoopOpt}
}

// WithArchitecture sets the platform option for crane.
//
// This option is actually a slight mis-use of the platform option, as it is
// setting the architecture only and hard coding the OS to linux.
func WithArchitecture(arch string) crane.Option {
	return crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: arch})
}

// CommonOpts returns a set of common options for crane under Zarf.
func CommonOpts(arch string) []crane.Option {
	opts := WithGlobalInsecureFlag()
	opts = append(opts, WithArchitecture(arch))

	opts = append(opts,
		crane.WithUserAgent("zarf"),
		crane.WithNoClobber(true),
		crane.WithJobs(1),
	)
	return opts
}

// WithBasicAuth returns an option for crane that sets basic auth.
func WithBasicAuth(username, password string) crane.Option {
	return crane.WithAuth(authn.FromConfig(authn.AuthConfig{
		Username: username,
		Password: password,
	}))
}

// WithPullAuth returns an option for crane that sets pull auth from a given registry info.
func WithPullAuth(ri types.RegistryInfo) crane.Option {
	return WithBasicAuth(ri.PullUsername, ri.PullPassword)
}

// WithPushAuth returns an option for crane that sets push auth from a given registry info.
func WithPushAuth(ri types.RegistryInfo) crane.Option {
	return WithBasicAuth(ri.PushUsername, ri.PushPassword)
}

func getSizeOfImage(manifestDesc ocispec.Descriptor, manifest ocispec.Manifest) int64 {
	var totalSize int64
	totalSize += manifestDesc.Size
	for _, layer := range manifest.Layers {
		totalSize += layer.Size
	}
	totalSize += manifest.Config.Size
	return totalSize
}
