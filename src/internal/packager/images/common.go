// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// PullConfig is the configuration for pulling images.
type PullConfig struct {
	OCIConcurrency        int
	DestinationDirectory  string
	ImageList             []transform.Image
	Arch                  string
	RegistryOverrides     map[string]string
	CacheDirectory        string
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
	ResponseHeaderTimeout time.Duration
}

// PushConfig is the configuration for pushing images.
type PushConfig struct {
	OCIConcurrency        int
	SourceDirectory       string
	ImageList             []transform.Image
	RegistryInfo          state.RegistryInfo
	NoChecksum            bool
	Arch                  string
	Retries               int
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
	Cluster               *cluster.Cluster
	ResponseHeaderTimeout time.Duration
}

const (
	//DockerMediaTypeManifest is the Legacy Docker manifest format, replaced by OCI manifest
	DockerMediaTypeManifest = "application/vnd.docker.distribution.manifest.v2+json"
	// DockerMediaTypeManifestList is the legacy Docker manifest list, replaced by OCI index
	DockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

// Legacy Docker image layers
const (
	DockerLayer             = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	DockerUncompressedLayer = "application/vnd.docker.image.rootfs.diff.tar"
	DockerForeignLayer      = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
)

func isLayer(mediaType string) bool {
	switch mediaType {
	//nolint: staticcheck // some of these layers are deprecated now, but they're included in this check since older images could still be using them
	case DockerLayer, DockerUncompressedLayer, ocispec.MediaTypeImageLayerGzip, ocispec.MediaTypeImageLayerZstd, ocispec.MediaTypeImageLayer,
		DockerForeignLayer, ocispec.MediaTypeImageLayerNonDistributableZstd, ocispec.MediaTypeImageLayerNonDistributable, ocispec.MediaTypeImageLayerNonDistributableGzip:
		return true
	}
	return false
}

// OnlyHasImageLayers returns true when an OCI manifest only containers container image layers.
func OnlyHasImageLayers(manifest ocispec.Manifest) bool {
	for _, layer := range manifest.Layers {
		if !isLayer(string(layer.MediaType)) {
			return false
		}
	}
	return true
}

func buildScheme(plainHTTP bool) string {
	if plainHTTP {
		return "http"
	}
	return "https"
}

// Ping verifies if a user can connect to a registry
func Ping(ctx context.Context, plainHTTP bool, registryURL string, client *auth.Client) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	url := fmt.Sprintf("%s://%s/v2/", buildScheme(plainHTTP), registryURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusUnauthorized, http.StatusForbidden:
		return nil
	}
	return fmt.Errorf("could not connect to registry %s over %s. status code: %d", registryURL, buildScheme(plainHTTP), resp.StatusCode)
}

// ShouldUsePlainHTTP returns true if the registryURL is an http endpoint
// This is inspired by the Crane functionality to determine the schema to be used - https://github.com/google/go-containerregistry/blob/main/pkg/v1/remote/transport/ping.go
// Zarf relies heavily on this logic, as the internal registry communicates over HTTP, however we want Zarf to be flexible should the registry be over https in the future
func ShouldUsePlainHTTP(ctx context.Context, registryURL string, client *auth.Client) (bool, error) {
	// If the https connection works use https
	err := Ping(ctx, false, registryURL, client)
	if err == nil {
		return false, nil
	}
	logger.From(ctx).Debug("failing back to plainHTTP connection", "registry_url", registryURL)
	// If https regular request failed and plainHTTP is allowed check again over plainHTTP
	err2 := Ping(ctx, true, registryURL, client)
	if err2 != nil {
		return false, errors.Join(err, err2)
	}
	return true, nil
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
	err = os.WriteFile(idxPath, b, helpers.ReadAllWriteUser)
	if err != nil {
		return fmt.Errorf("failed to save changes to index.json: %w", err)
	}
	return nil
}

func orasTransport(insecureSkipTLSVerify bool, responseHeaderTimeout time.Duration) (*retry.Transport, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("could not get default transport")
	}
	transport = transport.Clone()
	// Enable / Disable TLS verification based on the config
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureSkipTLSVerify}
	// Users frequently run into servers hanging indefinitely, if the server doesn't send headers in 10 seconds then we timeout to avoid this
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	return retry.NewTransport(transport), nil
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

// WithBasicAuth returns an option for crane that sets basic auth.
func WithBasicAuth(username, password string) crane.Option {
	return crane.WithAuth(authn.FromConfig(authn.AuthConfig{
		Username: username,
		Password: password,
	}))
}

// WithPullAuth returns an option for crane that sets pull auth from a given registry info.
func WithPullAuth(ri state.RegistryInfo) crane.Option {
	return WithBasicAuth(ri.PullUsername, ri.PullPassword)
}

// WithPushAuth returns an option for crane that sets push auth from a given registry info.
func WithPushAuth(ri state.RegistryInfo) crane.Option {
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
