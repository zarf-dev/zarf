// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	if cfg.ResponseHeaderTimeout <= 0 {
		cfg.ResponseHeaderTimeout = 10 * time.Second
	}
	cfg.ImageList = helpers.Unique(cfg.ImageList)
	toPush := map[string]struct{}{}
	for _, img := range cfg.ImageList {
		toPush[img.Reference] = struct{}{}
	}
	l := logger.From(ctx)
	registryURL := cfg.RegistryInfo.Address
	err := addRefNameAnnotationToImages(cfg.SourceDirectory)
	if err != nil {
		return err
	}

	src, err := oci.NewWithContext(ctx, cfg.SourceDirectory)
	if err != nil {
		return fmt.Errorf("failed to instantiate oci directory: %w", err)
	}

	// The user may or may not have a cluster available, if it's available then use it to connect to the registry
	c, _ := cluster.New(ctx)
	err = retry.Do(func() error {
		// reset concurrency to user-provided value on each component retry
		ociConcurrency := cfg.OCIConcurrency

		// Include tunnel connection in case the port forward breaks, for example, a registry pod could spin down / restart
		var tunnel *cluster.Tunnel
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

		client.Client.Transport = orasTransport(cfg.InsecureSkipTLSVerify, cfg.ResponseHeaderTimeout)

		plainHTTP := cfg.PlainHTTP

		if dns.IsLocalhost(registryURL) && !cfg.PlainHTTP {
			var err error
			plainHTTP, err = shouldUsePlainHTTP(ctx, registryURL, client)
			if err != nil {
				return err
			}
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
					return copyImage(ctx, src, remoteRepo, srcName, dstName, ociConcurrency, defaultPlatform)
				})
			}
			return copyImage(ctx, src, remoteRepo, srcName, dstName, ociConcurrency, defaultPlatform)
		}
		pushed := []string{}
		// Delete the images that were already successfully pushed so that they aren't attempted on the next retry
		defer func() {
			for _, refInfo := range pushed {
				delete(toPush, refInfo)
			}
		}()
		for img := range toPush {
			l.Info("pushing image", "name", img)
			// If this is not a no checksum image push it for use with the Zarf agent
			if !cfg.NoChecksum {
				offlineNameCRC, err := transform.ImageTransformHost(registryURL, img)
				if err != nil {
					return err
				}

				err = retry.Do(
					func() error { return pushImage(img, offlineNameCRC) },
					retry.OnRetry(func(_ uint, err error) {
						ociConcurrency = 1
						l.Debug("retrying image push", "error", err, "concurrency", ociConcurrency)
					}),
					retry.Context(ctx),
					retry.Attempts(2),
					retry.Delay(500*time.Millisecond),
				)
				if err != nil {
					return err
				}
			}

			// To allow for other non-zarf workloads to easily see the images upload a non-checksum version
			// (this may result in collisions but this is acceptable for this use case)
			offlineName, err := transform.ImageTransformHostWithoutChecksum(registryURL, img)
			if err != nil {
				return err
			}

			err = retry.Do(
				func() error { return pushImage(img, offlineName) },
				retry.OnRetry(func(_ uint, err error) {
					ociConcurrency = 1
					l.Debug("retrying image push", "error", err, "concurrency", ociConcurrency)
				}),
				retry.Context(ctx),
				retry.Attempts(2),
				retry.Delay(500*time.Millisecond),
			)
			if err != nil {
				return err
			}

			pushed = append(pushed, img)
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(uint(cfg.Retries)), retry.Delay(500*time.Millisecond), retry.OnRetry(func(attempt uint, _ error) {
		if uint(cfg.Retries) > 2 && attempt == uint(cfg.Retries)-2 {
			cfg.ResponseHeaderTimeout = 60 * time.Second // this should really never happen
		}
		l.Debug("retrying component image(s) push", "response_timeout", cfg.ResponseHeaderTimeout)
	}))
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
	l := logger.From(ctx)
	// Assume no platform to start as it can be nil in non container image situations
	resolveOpts := oras.DefaultResolveOptions
	desc, err := oras.Resolve(ctx, src, srcName, resolveOpts)
	if err != nil {
		return fmt.Errorf("failed to resolve image: %s: %w", srcName, err)
	}

	// checking the local package instead of preforming a network call to the remote registry
	_, b, err := oras.FetchBytes(ctx, src, srcName, oras.DefaultFetchBytesOptions)
	if err != nil {
		return fmt.Errorf("failed to resolve image manifest: %s: %w", srcName, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return err
	}

	l.Debug("artifact info", "MediaType", manifest.Config.MediaType, "dstName", dstName)

	// If an index is pulled we should try pulling with the default platform
	if isIndex(desc.MediaType) {
		resolveOpts.TargetPlatform = defaultPlatform
		desc, err = oras.Resolve(ctx, src, srcName, resolveOpts)
		if err != nil {
			return fmt.Errorf("failed to resolve image %s with architecture %s: %w", srcName, defaultPlatform.Architecture, err)
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

	if isChart(manifest.Config.MediaType) {
		l.Debug("annotation", "key", ocispec.AnnotationVersion, "value", manifest.Annotations[ocispec.AnnotationVersion])

		// split on the last
		tag := strings.LastIndex(dstName, ":")
		if manifest.Annotations[ocispec.AnnotationVersion] != dstName[tag+1:] {
			manifest.Annotations[ocispec.AnnotationVersion] = dstName[tag+1:]
			l.Debug("need to change the " + ocispec.AnnotationVersion + " annotation")
			bytes, err := json.Marshal(manifest)
			if err != nil {
				return fmt.Errorf("failed parse %s: %w", srcName, err)
			}
			l.Debug(string(bytes))
			_, err = oras.TagBytes(ctx, remote, ocispec.MediaTypeImageManifest, bytes, dstName)
			if err != nil {
				return fmt.Errorf("failed to push image %s: %w", srcName, err)
			}
		}
	}

	return nil
}
