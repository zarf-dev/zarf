// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	orasRetry "oras.land/oras-go/v2/registry/remote/retry"

	"github.com/defenseunicorns/pkg/helpers/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

const defaultRetries = 3

// Push pushes images to a registry.
func Push(ctx context.Context, cfg PushConfig) error {
	if cfg.Retries < 1 {
		cfg.Retries = defaultRetries
	}
	cfg.ImageList = helpers.Unique(cfg.ImageList)
	l := logger.From(ctx)
	registryURL := cfg.RegistryInfo.Address
	var tunnel *cluster.Tunnel
	c, _ := cluster.NewCluster()
	if c != nil {
		var err error
		registryURL, tunnel, err = c.ConnectToZarfRegistryEndpoint(ctx, cfg.RegistryInfo)
		if err != nil {
			return err
		}
		if tunnel != nil {
			defer tunnel.Close()
		}
	}
	client := &auth.Client{
		Client: orasRetry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(registryURL, auth.Credential{
			Username: cfg.RegistryInfo.PushUsername,
			Password: cfg.RegistryInfo.PushPassword,
		}),
	}

	plainHTTP := cfg.PlainHTTP

	if dns.IsLocalhost(registryURL) && !cfg.PlainHTTP {
		var err error
		plainHTTP, err = shouldUsePlainHTTP(ctx, registryURL, client)
		if err != nil {
			return err
		}
	}
	err := addRefNameAnnotationToImages(cfg.SourceDirectory)
	if err != nil {
		return err
	}

	src, err := oci.NewWithContext(ctx, cfg.SourceDirectory)
	if err != nil {
		return fmt.Errorf("failed to instantiate oci directory: %w", err)
	}

	pushImage := func(srcName, dstName string) error {
		remoteRepo := &orasRemote.Repository{
			PlainHTTP: plainHTTP,
			Client:    client,
		}
		remoteRepo.Reference, err = registry.ParseReference(dstName)
		if err != nil {
			return fmt.Errorf("failed to parse ref %s: %w", dstName, err)
		}
		defaultPlatform := &ocispec.Platform{
			Architecture: cfg.Arch,
			OS:           "linux",
		}
		if tunnel != nil {
			return tunnel.Wrap(func() error {
				return copyImage(ctx, src, remoteRepo, srcName, dstName, cfg.OCIConcurrency, defaultPlatform)
			})
		}
		return copyImage(ctx, src, remoteRepo, srcName, dstName, cfg.OCIConcurrency, defaultPlatform)
	}

	err = retry.Do(func() error {
		for _, img := range cfg.ImageList {
			l.Info("pushing image", "name", img.Reference)
			// If this is not a no checksum image push it for use with the Zarf agent
			if !cfg.NoChecksum {
				offlineNameCRC, err := transform.ImageTransformHost(registryURL, img.Reference)
				if err != nil {
					return err
				}

				if err = pushImage(img.Reference, offlineNameCRC); err != nil {
					return err
				}
			}

			// To allow for other non-zarf workloads to easily see the images upload a non-checksum version
			// (this may result in collisions but this is acceptable for this use case)
			offlineName, err := transform.ImageTransformHostWithoutChecksum(registryURL, img.Reference)
			if err != nil {
				return err
			}

			if err = pushImage(img.Reference, offlineName); err != nil {
				return err
			}
			return nil
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(uint(cfg.Retries)), retry.Delay(500*time.Millisecond))
	if err != nil {
		return err
	}

	return nil
}

func addRefNameAnnotationToImages(ociLayoutDirectory string) error {
	idx, err := getIndexFromOCILayout(ociLayoutDirectory)
	if err != nil {
		return err
	}
	// Crane sets ocispec.AnnotationBaseImageName instead of ocispec.AnnotationRefName
	// which ORAS uses to find images. We do this to be backwards compatible with packages built with Crane
	var correctedManifests []ocispec.Descriptor
	for _, manifest := range idx.Manifests {
		if manifest.Annotations[ocispec.AnnotationRefName] == "" {
			manifest.Annotations[ocispec.AnnotationRefName] = manifest.Annotations[ocispec.AnnotationBaseImageName]
		}
		correctedManifests = append(correctedManifests, manifest)
	}
	idx.Manifests = correctedManifests
	err = saveIndexToOCILayout(ociLayoutDirectory, idx)
	if err != nil {
		return err
	}
	return nil
}

func copyImage(ctx context.Context, src *oci.Store, remote oras.Target, srcName string, dstName string, concurrency int, defaultPlatform *ocispec.Platform) error {
	// Assume no platform to start as it can be nil in non container image situations
	resolveOpts := oras.DefaultResolveOptions
	desc, err := oras.Resolve(ctx, src, srcName, resolveOpts)
	if err != nil {
		return fmt.Errorf("failed to fetch image: %s: %w", srcName, err)
	}

	// If an index is pulled we should try pulling with the default platform
	if isIndex(desc.MediaType) {
		resolveOpts.TargetPlatform = defaultPlatform
		desc, err = oras.Resolve(ctx, src, srcName, resolveOpts)
		if err != nil {
			return fmt.Errorf("failed to fetch image %s with architecture %s: %w", srcName, defaultPlatform.Architecture, err)
		}
	}

	if !isManifest(desc.MediaType) {
		return fmt.Errorf("expected OCI manifest got %s", desc.MediaType)
	}

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = concurrency
	copyOpts.WithTargetPlatform(desc.Platform)
	_, err = oras.Copy(ctx, src, srcName, remote, dstName, copyOpts)
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", srcName, err)
	}
	return nil
}
