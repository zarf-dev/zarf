// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"

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

	// TODO, go through cfg source directory and make the annotations the same between them.


	src, err := oci.NewWithContext(ctx, cfg.SourceDirectory)
	if err != nil {
		return fmt.Errorf("failed to instantiate oci directory: %w", err)
	}

	pushImage := func(srcName, dstName string) error {
		remoteRepo := &orasRemote.Repository{
			PlainHTTP: cfg.PlainHTTP,
			Client:    client,
		}
		copyOpts := oras.DefaultCopyOptions
		p := &ocispec.Platform{
			OS:           "linux",
			Architecture: cfg.Arch,
		}
		remoteRepo.Reference, err = registry.ParseReference(dstName)
		if err != nil {
			return fmt.Errorf("failed to parse ref %s: %w", dstName, err)
		}
		copyOpts.WithTargetPlatform(p)
		if tunnel != nil {
			return tunnel.Wrap(func() error {
				remoteRepo.PlainHTTP = true
				_, err := oras.Copy(ctx, src, srcName, remoteRepo, dstName, copyOpts)
				if err != nil {
					return fmt.Errorf("failed to push image %s: %s: %w", srcName, dstName, err)
				}
				return err
			})
		}
		_, err := oras.Copy(ctx, src, srcName, remoteRepo, dstName, copyOpts)
		if err != nil {
			return fmt.Errorf("failed to push image %s: %w", srcName, err)
		}
		return err
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
