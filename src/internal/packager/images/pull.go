// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/client"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/defenseunicorns/pkg/helpers/v2"
	v1 "github.com/google/go-containerregistry/pkg/v1"
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

func pullFromDockerDaemon(ctx context.Context, images []transform.Image, dst oras.Target, arch string) (map[transform.Image]ocispec.Manifest, error) {
	imagesWithManifests := map[transform.Image]ocispec.Manifest{}
	dockerEndPointHost, err := getDockerEndpointHost()
	if err != nil {
		return nil, err
	}
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpDir)
	for _, image := range images {
		cli, err := client.NewClientWithOpts(
			client.WithHost(dockerEndPointHost),
			client.WithTLSClientConfigFromEnv(),
			client.WithVersionFromEnv(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Docker client: %w", err)
		}
		defer cli.Close()
		cli.NegotiateAPIVersion(ctx)
		// Note: ImageSave accepts a ocispec.Platform, BUT it would require users have docker engine API version 1.48
		// which was released in Feb 2025. This could make the code more efficient in some cases, but we are
		// avoiding this for now to give users more time to update.
		imageReader, err := cli.ImageSave(ctx, []string{image.Reference})
		if err != nil {
			return nil, fmt.Errorf("failed to save image: %w", err)
		}
		defer imageReader.Close()

		imageTarPath := filepath.Join(tmpDir, "image.tar")
		tarFile, err := os.Create(imageTarPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create tar file: %w", err)
		}
		defer tarFile.Close()

		// Read bytes from imageReader and write them to tarFile
		if _, err := io.Copy(tarFile, imageReader); err != nil {
			return nil, fmt.Errorf("error writing image to tar file: %w", err)
		}
		dockerImageOCILayoutPath := filepath.Join(tmpDir, "docker-image-oci-layout")
		if err := archiver.Unarchive(imageTarPath, dockerImageOCILayoutPath); err != nil {
			return nil, fmt.Errorf("failed to write tar file: %w", err)
		}

		b, err := os.ReadFile(filepath.Join(dockerImageOCILayoutPath, "index.json"))
		if err != nil {
			return nil, fmt.Errorf("failed to read index.json: %w", err)
		}
		var index ocispec.Index
		if err := json.Unmarshal(b, &index); err != nil {
			return nil, fmt.Errorf("failed to unmarshal index.json: %w", err)
		}
		// Indexes should always contain exactly one manifests for the single image we are pulling
		if len(index.Manifests) != 1 {
			return nil, fmt.Errorf("index.json does not contain one manifest")
		}
		// Docker does set the annotation ref name in the way ORAS anticipates
		// We set it here so that ORAS can pick up the image
		index.Manifests[0].Annotations[ocispec.AnnotationRefName] = image.Reference
		b, err = json.Marshal(index)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal index.json: %w", err)
		}
		err = os.WriteFile(filepath.Join(dockerImageOCILayoutPath, "index.json"), b, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed to write index.json: %w", err)
		}

		dockerImageSrc, err := oci.New(dockerImageOCILayoutPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create OCI store: %w", err)
		}

		fetchBytesOpts := oras.DefaultFetchBytesOptions
		platform := &ocispec.Platform{
			Architecture: arch,
			OS:           "linux",
		}
		fetchBytesOpts.TargetPlatform = platform
		desc, b, err := oras.FetchBytes(ctx, dockerImageSrc, image.Reference, fetchBytesOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to get manifest from docker image source: %w", err)
		}
		if !(desc.MediaType == ocispec.MediaTypeImageManifest || desc.MediaType == DockerMediaTypeManifest) {
			return nil, fmt.Errorf("expected to find image manifest instead found %s", desc.MediaType)
		}
		var manifest ocispec.Manifest
		if err := json.Unmarshal(b, &manifest); err != nil {
			return nil, err
		}
		imagesWithManifests[image] = manifest
		copyOpts := oras.DefaultCopyOptions
		copyOpts.WithTargetPlatform(platform)
		_, err = oras.Copy(ctx, dockerImageSrc, image.Reference, dst, "", copyOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to copy: %w", err)
		}
	}
	return imagesWithManifests, nil
}

// Pull pulls all images from the given config.
func Pull(ctx context.Context, cfg PullConfig) (map[transform.Image]ocispec.Manifest, error) {
	l := logger.From(ctx)
	pullStart := time.Now()

	imageCount := len(cfg.ImageList)
	if err := helpers.CreateDirectory(cfg.DestinationDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", cfg.DestinationDirectory, err)
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
	platform := &ocispec.Platform{
		Architecture: cfg.Arch,
		OS:           "linux",
	}
	imagesWithManifests := map[transform.Image]ocispec.Manifest{}
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

		// If the image has a digest start out by checking if it's an index sha
		if image.Digest != "" {
			desc, b, err := oras.FetchBytes(ctx, localRepo, image.Reference, oras.DefaultFetchBytesOptions)
			if err != nil {
				return nil, err
			}
			if desc.MediaType == ocispec.MediaTypeImageIndex || desc.MediaType == DockerMediaTypeManifestList {
				// Both index types can be marshalled into an ocispec.Index
				// https://github.com/oras-project/oras-go/blob/853e0125ccad32ff691e4ed70e156c7619021bfd/internal/manifestutil/parser.go#L55
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

		fetchOpts := oras.DefaultFetchBytesOptions
		fetchOpts.FetchOptions.TargetPlatform = platform
		desc, b, err := oras.FetchBytes(ctx, localRepo, image.Reference, fetchOpts)
		if err != nil {
			// If the image was not found it could be an image signature or Helm image
			// In this case we can check if the image was not found by using default fetch byte options
			// If the image is found this time we assume that it is not a container image
			desc, b, err = oras.FetchBytes(ctx, localRepo, image.Reference, oras.DefaultFetchBytesOptions)
			if err != nil {
				// If the image is not found again then we should try to pull it from the daemon
				dockerFallBack = append(dockerFallBack, image)
				continue
			}
		}
		if desc.MediaType == ocispec.MediaTypeImageManifest || desc.MediaType == DockerMediaTypeManifest {
			// Both manifest types can be marshalled into a manifest
			// https://github.com/oras-project/oras-go/blob/853e0125ccad32ff691e4ed70e156c7619021bfd/internal/manifestutil/parser.go#L37
			var manifest ocispec.Manifest
			if err := json.Unmarshal(b, &manifest); err != nil {
				return nil, err
			}
			ImagesWithDescriptors[image] = desc
			imagesWithManifests[image] = manifest
		} else {
			return nil, fmt.Errorf("received unexpected mediatype %s", desc.MediaType)
		}
	}
	l.Debug("done fetching info for images", "count", len(cfg.ImageList), "duration", time.Since(imageFetchStart))

	l.Info("pulling images", "count", len(cfg.ImageList))

	dst, err := oci.NewWithContext(ctx, cfg.DestinationDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to create oci formatted directory: %w", err)
	}

	if len(dockerFallBack) > 0 {
		daemonImagesWithManifests, err := pullFromDockerDaemon(ctx, dockerFallBack, dst, cfg.Arch)
		if err != nil {
			return nil, fmt.Errorf("failed to pull images from docker: %w", err)
		}
		for k, v := range daemonImagesWithManifests {
			imagesWithManifests[k] = v
		}
	}

	// TODO need to see if this is still an issue
	// Needed because when pulling from the local docker daemon, while using the docker containerd runtime
	// Crane incorrectly names the blob of the docker image config to a sha that does not match the contents
	// https://github.com/zarf-dev/zarf/issues/2584
	// This is a band aid fix while we wait for crane and or docker to create the permanent fix

	err = orasSave(ctx, ImagesWithDescriptors, cfg, dst, platform, client)
	if err != nil {
		return nil, fmt.Errorf("failed to save images: %w", err)
	}

	l.Debug("done pulling images", "count", len(cfg.ImageList), "duration", time.Since(pullStart))

	return imagesWithManifests, nil
}

func orasSave(ctx context.Context, ImagesWithDescriptors map[transform.Image]ocispec.Descriptor, cfg PullConfig, dst oras.Target, platform *ocispec.Platform, client *auth.Client) error {
	l := logger.From(ctx)
	for image, desc := range ImagesWithDescriptors {
		var pullSrc oras.ReadOnlyTarget
		var err error
		remoteRepo := &orasRemote.Repository{PlainHTTP: cfg.PlainHTTP}
		remoteRepo.Reference, err = registry.ParseReference(image.Reference)
		if err != nil {
			return fmt.Errorf("failed to parse image reference %s: %w", image.Reference, err)
		}
		remoteRepo.Client = client

		// TODO add size in bytes
		copyOpts := oras.DefaultCopyOptions

		copyOpts.WithTargetPlatform(desc.Platform)
		l.Info("saving image", "ref", image.Reference, "method", "sequential")
		if cfg.CacheDirectory == "" {
			pullSrc = remoteRepo
		} else {
			localCache, err := oci.NewWithContext(ctx, cfg.CacheDirectory)
			if err != nil {
				return fmt.Errorf("failed to create oci formatted directory: %w", err)
			}
			pullSrc = orasCache.New(remoteRepo, localCache)
		}
		_, err = oras.Copy(ctx, pullSrc, image.Reference, dst, "", copyOpts)
		if err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
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
