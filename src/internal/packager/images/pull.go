// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/flags"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	clayout "github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	orasCache "github.com/zarf-dev/zarf/src/internal/packager/images/cache"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"golang.org/x/sync/errgroup"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func checkForIndex(refInfo transform.Image, desc *remote.Descriptor) error {
	if refInfo.Digest != "" && desc != nil && types.MediaType(desc.MediaType).IsIndex() {
		var idx v1.IndexManifest
		if err := json.Unmarshal(desc.Manifest, &idx); err != nil {
			return fmt.Errorf("unable to unmarshal index.json: %w", err)
		}
		lines := []string{"The following images are available in the index:"}
		name := refInfo.Name
		if refInfo.Tag != "" {
			name += ":" + refInfo.Tag
		}
		for _, desc := range idx.Manifests {
			lines = append(lines, fmt.Sprintf("image - %s@%s with platform %s", name, desc.Digest.String(), desc.Platform.String()))
		}
		imageOptions := strings.Join(lines, "\n")
		return fmt.Errorf("%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use: %s", refInfo.Reference, imageOptions)
	}
	return nil
}

func getDockerEndpointHost() (string, error) {
	dockerCli, err := command.NewDockerCli(command.WithStandardStreams())
	if err != nil {
		return "", err
	}
	newClientOpts := flags.NewClientOptions()
	err = dockerCli.Initialize(newClientOpts)
	if err != nil {
		return "", err
	}
	store := dockerCli.ContextStore()
	metadata, err := store.GetMetadata(dockerCli.CurrentContext())
	if err != nil {
		return "", err
	}
	endpoint, err := docker.EndpointFromContext(metadata)
	if err != nil {
		return "", err
	}
	return endpoint.Host, nil
}

// Pull pulls all images from the given config.
func Pull(ctx context.Context, cfg PullConfig) ([]ocispec.Manifest, error) {
	l := logger.From(ctx)
	pullStart := time.Now()

	imageCount := len(cfg.ImageList)
	if err := helpers.CreateDirectory(cfg.DestinationDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", cfg.DestinationDirectory, err)
	}

	dst, err := oci.NewWithContext(ctx, cfg.DestinationDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to create oci formatted directory: %w", err)
	}

	// Give some additional user feedback on larger image sets
	imageFetchStart := time.Now()
	l.Info("fetching info for images", "count", imageCount, "destination", cfg.DestinationDirectory)
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}
	client := &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}
	imageManifests := []ocispec.Manifest{}
	ImagesWithDescriptors := map[transform.Image]ocispec.Descriptor{}
	dockerFallBack := []transform.Image{}

	// This loop pulls the metadata from images with three goals
	// - discover if any images are sha'd to an index, if so error which options for different platforms
	// - If the repo doesn't contain an image mark them so that we can try to pull them from the daemon instead
	// - Get all the manifests from images that will be pulled so they can be returned
	for _, image := range cfg.ImageList {
		localRepo := &orasRemote.Repository{PlainHTTP: true}
		var err error

		localRepo.Reference, err = registry.ParseReference(image.Reference)
		if err != nil {
			return nil, err
		}

		localRepo.Client = client

		// It's not going to be an index unless a digest is used, but even if a digest is used if we set the platform then a manifest will be used
		// because of this if a digest is used we have to first check if it's an index if it is not then we continue. Otherwise we error
		fetchOpts := oras.DefaultFetchBytesOptions
		fetchOpts.FetchOptions.TargetPlatform = &ocispec.Platform{
			Architecture: cfg.Arch,
			OS:           "linux",
		}
		desc, b, err := oras.FetchBytes(ctx, localRepo, image.Reference, fetchOpts)
		if err != nil {
			fmt.Println("falling back to docker")
			dockerFallBack = append(dockerFallBack, image)
			continue
		}
		if image.Digest != "" {
			desc, b, err := oras.FetchBytes(ctx, localRepo, image.Reference, oras.DefaultFetchBytesOptions)
			if err != nil {
				return nil, err
			}
			// TODO case statement this
			if desc.MediaType == ocispec.MediaTypeImageIndex || desc.MediaType == DockerMediaTypeManifestList {
				var idx ocispec.Index
				if err := json.Unmarshal(b, &idx); err != nil {
					return nil, fmt.Errorf("unable to unmarshal index.json: %w", err)
				}
				lines := []string{"The following images are available in the index:"}
				name := image.Name
				if image.Tag != "" {
					name += ":" + image.Tag
				}
				for _, desc := range idx.Manifests {
					lines = append(lines, fmt.Sprintf("image - %s@%s with platform %s", name, desc.Digest, desc.Platform))
				}
				imageOptions := strings.Join(lines, "\n")
				return nil, fmt.Errorf("%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use: %s", image.Reference, imageOptions)
			}
		}

		if desc.MediaType == ocispec.MediaTypeImageManifest || desc.MediaType == DockerMediaTypeManifest {
			var manifest ocispec.Manifest
			if err := json.Unmarshal(b, &manifest); err != nil {
				return nil, err
			}
			ImagesWithDescriptors[image] = desc
			imageManifests = append(imageManifests, manifest)
		} else {
			return nil, fmt.Errorf("received unexpected mediatype %s", desc.MediaType)
		}
	}
	l.Debug("done fetching info for images", "count", len(cfg.ImageList), "duration", time.Since(imageFetchStart))

	l.Info("pulling images", "count", len(cfg.ImageList))

	if len(dockerFallBack) > 0 {
		return nil, fmt.Errorf("docker not implemented yet")
	}

	// TODO need to see if this is still an issue
	// Needed because when pulling from the local docker daemon, while using the docker containerd runtime
	// Crane incorrectly names the blob of the docker image config to a sha that does not match the contents
	// https://github.com/zarf-dev/zarf/issues/2584
	// This is a band aid fix while we wait for crane and or docker to create the permanent fix

	err = orasSave(ctx, ImagesWithDescriptors, dst, cfg.CacheDirectory, client)
	if err != nil {
		return nil, fmt.Errorf("failed to save images: %w", err)
	}

	l.Debug("done pulling images", "count", len(cfg.ImageList), "duration", time.Since(pullStart))

	return imageManifests, nil
}

func orasSave(ctx context.Context, images map[transform.Image]ocispec.Descriptor, dst oras.Target, cachePath string, client *auth.Client) error {
	l := logger.From(ctx)
	for image, desc := range images {
		var err error
		localRepo := &orasRemote.Repository{PlainHTTP: true}
		localRepo.Reference, err = registry.ParseReference(image.Reference)
		if err != nil {
			return err
		}
		localRepo.Client = client
		// TODO fix this
		if cachePath == "" {
			cachePath = "/tmp/images"
		}
		localCache, err := oci.NewWithContext(ctx, cachePath)
		if err != nil {
			return fmt.Errorf("failed to create oci formatted directory: %w", err)
		}

		// TODO add size in bytes
		copyOpts := oras.DefaultCopyOptions
		copyOpts.WithTargetPlatform(desc.Platform)
		l.Info("saving image", "ref", image.Reference, "method", "sequential")
		remoteWithCache := orasCache.New(localRepo, localCache)
		desc, err := oras.Copy(ctx, remoteWithCache, image.Reference, dst, "", copyOpts)
		if err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
		fmt.Println("finished copying image", desc.Digest, desc.Platform)
	}
	return nil
}

// from https://github.com/google/go-containerregistry/blob/6bce25ecf0297c1aa9072bc665b5cf58d53e1c54/pkg/v1/cache/fs.go#L143
func layerCachePath(path string, h v1.Hash) string {
	var file string
	if runtime.GOOS == "windows" {
		file = fmt.Sprintf("%s-%s", h.Algorithm, h.Hex)
	} else {
		file = h.String()
	}
	return filepath.Join(path, file)
}

// CleanupInProgressLayers removes incomplete layers from the cache.
func CleanupInProgressLayers(ctx context.Context, img v1.Image, cacheDirectory string) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}
	eg, _ := errgroup.WithContext(ctx)
	for _, layer := range layers {
		layer := layer
		eg.Go(func() error {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}
			size, err := layer.Size()
			if err != nil {
				return err
			}
			location := layerCachePath(cacheDirectory, digest)
			info, err := os.Stat(location)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if info.Size() != size {
				if err := os.Remove(location); err != nil {
					return fmt.Errorf("failed to remove incomplete layer %s: %w", digest.Hex, err)
				}
			}
			return nil
		})
	}
	return eg.Wait()
}

func getSizeOfImage(img v1.Image) (int64, error) {
	var totalSize int64
	manifestSize, err := img.Size()
	if err != nil {
		return 0, err
	}
	totalSize += manifestSize
	manifest, err := img.Manifest()
	if err != nil {
		return 0, err
	}
	totalSize += manifest.Config.Size
	layers, err := img.Layers()
	if err != nil {
		return 0, err
	}
	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			return 0, err
		}
		totalSize += size
	}
	return totalSize, nil
}

// SaveSequential saves images sequentially.
func SaveSequential(ctx context.Context, cl clayout.Path, m map[transform.Image]v1.Image, cacheDirectory string) (map[transform.Image]v1.Image, error) {
	l := logger.From(ctx)
	saved := map[transform.Image]v1.Image{}
	for info, img := range m {
		annotations := map[string]string{
			ocispec.AnnotationBaseImageName: info.Reference,
		}
		wStart := time.Now()
		size, err := getSizeOfImage(img)
		if err != nil {
			return saved, fmt.Errorf("failed to get size of image: %w", err)
		}
		byteSize := utils.ByteFormat(float64(size), 2)
		l.Info("saving image", "ref", info.Reference, "size", byteSize, "method", "sequential")
		if err := cl.AppendImage(img, clayout.WithAnnotations(annotations)); err != nil {
			if err := CleanupInProgressLayers(ctx, img, cacheDirectory); err != nil {
				l.Error("failed to clean up in-progress layers. please run `zarf tools clear-cache`")
			}
			return saved, err
		}
		saved[info] = img
		l.Debug("done saving image",
			"ref", info.Reference,
			"bytes", size,
			"method", "sequential",
			"duration", time.Since(wStart),
		)
	}
	return saved, nil
}

// SaveConcurrent saves images in a concurrent, bounded manner.
func SaveConcurrent(ctx context.Context, cl clayout.Path, m map[transform.Image]v1.Image, cacheDirectory string) (map[transform.Image]v1.Image, error) {
	l := logger.From(ctx)
	saved := map[transform.Image]v1.Image{}

	var mu sync.Mutex

	eg, ectx := errgroup.WithContext(ctx)
	eg.SetLimit(10)

	for info, img := range m {
		info, img := info, img
		eg.Go(func() error {
			select {
			case <-ectx.Done():
				return ectx.Err()
			default:
				desc, err := partial.Descriptor(img)
				if err != nil {
					return err
				}
				size, err := getSizeOfImage(img)
				if err != nil {
					return err
				}
				byteSize := utils.ByteFormat(float64(size), 2)
				wStart := time.Now()
				l.Info("saving image", "ref", info.Reference, "size", byteSize, "method", "concurrent")
				if err := cl.WriteImage(img); err != nil {
					if err := CleanupInProgressLayers(ectx, img, cacheDirectory); err != nil {
						l.Error("failed to clean up in-progress layers. please run `zarf tools clear-cache`")
					}
					return err
				}
				l.Debug("done saving image",
					"ref", info.Reference,
					"bytes", size,
					"method", "concurrent",
					"duration", time.Since(wStart),
				)

				mu.Lock()
				defer mu.Unlock()
				annotations := map[string]string{
					ocispec.AnnotationBaseImageName: info.Reference,
				}
				desc.Annotations = annotations
				if err := cl.AppendDescriptor(*desc); err != nil {
					return err
				}

				saved[info] = img
				return nil
			}
		})
	}

	return saved, eg.Wait()
}
