// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// Push pushes images to a registry.
func Push(ctx context.Context, cfg PushConfig) error {
	l := logger.From(ctx)
	var (
		err         error
		tunnel      *cluster.Tunnel
		registryURL = cfg.RegInfo.Address
	)
	c, _ := cluster.NewCluster()
	if c != nil {
		registryURL, tunnel, err = c.ConnectToZarfRegistryEndpoint(ctx, cfg.RegInfo)
		if err != nil {
			return err
		}
		if tunnel != nil {
			defer tunnel.Close()
		}
	}

	client := &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(registryURL, auth.Credential{
			Username: cfg.RegInfo.PushUsername,
			Password: cfg.RegInfo.PushPassword,
		}),
	}
	idx, err := getIndexFromOCILayout(cfg.SourceDirectory)
	if err != nil {
		return err
	}
	var correctedManifests []ocispec.Descriptor
	for _, manifest := range idx.Manifests {
		// Crane does not set ocispec.AnnotationRefName which ORAS uses to find images
		if manifest.Annotations[ocispec.AnnotationRefName] == "" {
			manifest.Annotations[ocispec.AnnotationRefName] = manifest.Annotations[ocispec.AnnotationBaseImageName]
		}
		correctedManifests = append(correctedManifests, manifest)
	}
	idx.Manifests = correctedManifests
	err = saveIndexToOCILayout(cfg.SourceDirectory, idx)
	if err != nil {
		return err
	}

	src, err := oci.NewWithContext(ctx, cfg.SourceDirectory)
	if err != nil {
		return fmt.Errorf("failed to instantiate oci directory: %w", err)
	}

	pushImage := func(srcName, dstName string) error {
		remoteRepo := &orasRemote.Repository{
			PlainHTTP: cfg.PlainHTTP,
			Client:    client,
		}
		remoteRepo.Reference, err = registry.ParseReference(dstName)
		if err != nil {
			return fmt.Errorf("failed to parse ref %s: %w", dstName, err)
		}
		if tunnel != nil {
			return tunnel.Wrap(func() error {
				remoteRepo.PlainHTTP = true
				return copyImage(ctx, src, remoteRepo, srcName, dstName)
			})
		}
		return copyImage(ctx, src, remoteRepo, srcName, dstName)
	}

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

	}

	return nil
}

func copyImage(ctx context.Context, src *oci.Store, remote oras.Target, srcName string, dstName string) error {

	// We get the platform dynamically because it can be nil in non container image cases
	desc, _, err := oras.Fetch(ctx, src, srcName, oras.DefaultFetchOptions)
	if err != nil {
		return fmt.Errorf("failed to fetch image: %s: %w", srcName, err)
	}
	// we only allow manifests during pull, this allows us to get the platform from the descriptor
	if !isManifest(desc.MediaType) {
		return fmt.Errorf("only OCI manifests are supported in Zarf, got %s", desc.MediaType)
	}
	copyOpts := oras.DefaultCopyOptions
	// We get the
	copyOpts.WithTargetPlatform(desc.Platform)
	_, err = oras.Copy(ctx, src, srcName, remote, dstName, copyOpts)
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", srcName, err)
	}
	return nil
}
