// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/client"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/defenseunicorns/pkg/helpers/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	orasCache "github.com/zarf-dev/zarf/src/internal/packager/images/cache"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

type imagePullInfo struct {
	registryOverrideRef string
	ref                 string
	manifestDesc        ocispec.Descriptor
	byteSize            int64
}

type imageDaemonPullInfo struct {
	registryOverrideRef string
	image               transform.Image
}

// Pull pulls all images from the given config.
func Pull(ctx context.Context, cfg PullConfig) (map[transform.Image]ocispec.Manifest, error) {
	l := logger.From(ctx)
	pullStart := time.Now()

	imageCount := len(cfg.ImageList)
	if err := helpers.CreateDirectory(cfg.DestinationDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", cfg.DestinationDirectory, err)
	}

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
		// TODO: in the future we could support Windows images
		OS:           "linux",
	}
	imagesWithManifests := map[transform.Image]ocispec.Manifest{}
	imagesInfo := []imagePullInfo{}
	dockerFallBackImages := []imageDaemonPullInfo{}
	var fetchMu sync.Mutex

	// This loop pulls the metadata from images with three goals
	// - discover if any images are sha'd to an index, if so error and inform user on the different available platforms
	// - If the resolve fails mark the image for docker daemon pull instead
	// - Get all the manifests from images that will be pulled so they can be returned to the function
	eg, ectx := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for _, image := range cfg.ImageList {
		image := image
		eg.Go(func() error {
			localRepo := &orasRemote.Repository{PlainHTTP: cfg.PlainHTTP}

			overriddenRef := image.Reference
			for k, v := range cfg.RegistryOverrides {
				if strings.HasPrefix(image.Reference, k) {
					overriddenRef = strings.Replace(image.Reference, k, v, 1)
				}
			}

			localRepo.Reference, err = registry.ParseReference(overriddenRef)
			if err != nil {
				return err
			}

			localRepo.Client = client

			fetchOpts := oras.DefaultFetchBytesOptions
			desc, b, err := oras.FetchBytes(ectx, localRepo, overriddenRef, fetchOpts)
			if err != nil {
				// TODO we could use the k8s library for backoffs here - https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/util/wait/backoff.go
				if strings.Contains(err.Error(), "toomanyrequests") {
					return fmt.Errorf("rate limited by registry: %w", err)
				}
				l.Warn("unable to find image, attempting pull from docker daemon as fallback", "image", overriddenRef, "err", err)
				fetchMu.Lock()
				defer fetchMu.Unlock()
				dockerFallBackImages = append(dockerFallBackImages, imageDaemonPullInfo{
					image:               image,
					registryOverrideRef: overriddenRef,
				})
				return nil
			}

			// If the image has a digest make
			if image.Digest != "" {
				err := checkForIndex(ectx, localRepo, overriddenRef, image)
				if err != nil {
					return err
				}
			}
			// If a manifest was returned from FetchBytes, either it's a tag with only one image or it's a non container image
			// If it's not a manifest then we received an index and need to pull the manifest by platform
			if !isManifest(desc.MediaType) {
				fetchOpts.FetchOptions.TargetPlatform = platform
				// If the image was not found it could be a image signature or Helm image
				// non container images don't have platforms so we check using the default opts
				desc, b, err = oras.FetchBytes(ectx, localRepo, overriddenRef, fetchOpts)
				if err != nil {
					return fmt.Errorf("failed to fetch image with architecture %s: %w", platform.Architecture, err)
				}
			}

			// This should never be true, but it's extra validation before we marshall
			if !isManifest(desc.MediaType) {
				return fmt.Errorf("received unexpected mediatype %s", desc.MediaType)
			}
			// Both oci and docker manifest types can be marshalled into a manifest
			// https://github.com/oras-project/oras-go/blob/853e0125ccad32ff691e4ed70e156c7619021bfd/internal/manifestutil/parser.go#L37
			var manifest ocispec.Manifest
			if err := json.Unmarshal(b, &manifest); err != nil {
				return err
			}
			size := getSizeOfImage(desc, manifest)
			fetchMu.Lock()
			defer fetchMu.Unlock()
			imagesInfo = append(imagesInfo, imagePullInfo{
				registryOverrideRef: overriddenRef,
				ref:                 image.Reference,
				byteSize:            size,
				manifestDesc:        desc,
			})
			imagesWithManifests[image] = manifest
			l.Debug("pulled manifest for image", "name", overriddenRef)
			return nil

		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	l.Debug("done fetching info for images", "count", len(cfg.ImageList), "duration", time.Since(imageFetchStart))

	l.Info("pulling images", "count", len(cfg.ImageList))

	dst, err := oci.NewWithContext(ctx, cfg.DestinationDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to create oci formatted directory: %w", err)
	}

	if len(dockerFallBackImages) > 0 {
		daemonImagesWithManifests, err := pullFromDockerDaemon(ctx, dockerFallBackImages, dst, cfg.Arch)
		if err != nil {
			return nil, fmt.Errorf("failed to pull images from docker: %w", err)
		}
		for k, v := range daemonImagesWithManifests {
			imagesWithManifests[k] = v
		}
	}

	err = orasSave(ctx, imagesInfo, cfg, dst, client)
	if err != nil {
		return nil, fmt.Errorf("failed to save images: %w", err)
	}

	l.Debug("done pulling images", "count", len(cfg.ImageList), "duration", time.Since(pullStart))

	return imagesWithManifests, nil
}

func checkForIndex(ctx context.Context, repo *orasRemote.Repository, overriddenRef string, image transform.Image) error {
	desc, b, err := oras.FetchBytes(ctx, repo, overriddenRef, oras.DefaultFetchBytesOptions)
	if err != nil {
		return fmt.Errorf("failed to fetch bytes: %w", err)
	}
	if !isIndex(desc.MediaType) {
		return nil
	}
	// Both index types can be marshalled into an ocispec.Index
	// https://github.com/oras-project/oras-go/blob/853e0125ccad32ff691e4ed70e156c7619021bfd/internal/manifestutil/parser.go#L55
	var idx ocispec.Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return fmt.Errorf("unable to unmarshal index.json: %w", err)
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
	return fmt.Errorf("%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use: %s", overriddenRef, imageOptions)
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

func pullFromDockerDaemon(ctx context.Context, daemonPullInfo []imageDaemonPullInfo, dst *oci.Store, arch string) (map[transform.Image]ocispec.Manifest, error) {
	l := logger.From(ctx)
	imagesWithManifests := map[transform.Image]ocispec.Manifest{}
	dockerEndPointHost, err := getDockerEndpointHost()
	if err != nil {
		return nil, err
	}
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
	for _, pullInfo := range daemonPullInfo {
		err := func() error {
			tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)
			// Note: ImageSave accepts a ocispec.Platform, but the effects it would have on users without client API version 1.48,
			// which was released in Feb 2025, is unclear. This could make the code more efficient in some cases, but we are
			// avoiding this for now to give users more time to update.
			imageReader, err := cli.ImageSave(ctx, []string{pullInfo.registryOverrideRef})
			if err != nil {
				return fmt.Errorf("failed to save image: %w", err)
			}
			defer imageReader.Close()

			imageTarPath := filepath.Join(tmpDir, "image.tar")
			tarFile, err := os.Create(imageTarPath)
			if err != nil {
				return fmt.Errorf("failed to create tar file: %w", err)
			}
			defer tarFile.Close()

			// Read bytes from imageReader and write them to tarFile
			if _, err := io.Copy(tarFile, imageReader); err != nil {
				return fmt.Errorf("error writing image to tar file: %w", err)
			}
			dockerImageOCILayoutPath := filepath.Join(tmpDir, "docker-image-oci-layout")
			if err := archiver.Unarchive(imageTarPath, dockerImageOCILayoutPath); err != nil {
				return fmt.Errorf("failed to write tar file: %w", err)
			}
			idx, err := getIndexFromOCILayout(dockerImageOCILayoutPath)
			if err != nil {
				return err
			}
			// Indexes should always contain exactly one manifests for the single image we are pulling
			if len(idx.Manifests) != 1 {
				return fmt.Errorf("index.json does not contain one manifest")
			}
			if idx.Manifests[0].Annotations == nil {
				idx.Manifests[0].Annotations = map[string]string{}
			}
			// Docker does set the annotation ref name in the way ORAS anticipates
			// We set it here so that ORAS can pick up the image
			idx.Manifests[0].Annotations[ocispec.AnnotationRefName] = pullInfo.registryOverrideRef
			err = saveIndexToOCILayout(dockerImageOCILayoutPath, idx)
			if err != nil {
				return err
			}

			dockerImageSrc, err := oci.New(dockerImageOCILayoutPath)
			if err != nil {
				return fmt.Errorf("failed to create OCI store: %w", err)
			}

			fetchBytesOpts := oras.DefaultFetchBytesOptions
			platform := &ocispec.Platform{
				Architecture: arch,
				OS:           "linux",
			}
			fetchBytesOpts.TargetPlatform = platform
			desc, b, err := oras.FetchBytes(ctx, dockerImageSrc, pullInfo.registryOverrideRef, fetchBytesOpts)
			if err != nil {
				return fmt.Errorf("failed to get manifest from docker image source: %w", err)
			}
			if !isManifest(desc.MediaType) {
				return fmt.Errorf("expected to find image manifest instead found %s", desc.MediaType)
			}
			var manifest ocispec.Manifest
			if err := json.Unmarshal(b, &manifest); err != nil {
				return err
			}
			imagesWithManifests[pullInfo.image] = manifest
			size := getSizeOfImage(desc, manifest)
			l.Info("pulling image from docker daemon", "name", pullInfo.registryOverrideRef, "size", utils.ByteFormat(float64(size), 2))
			copyOpts := oras.DefaultCopyOptions
			copyOpts.WithTargetPlatform(platform)
			manifestDesc, err := oras.Copy(ctx, dockerImageSrc, pullInfo.registryOverrideRef, dst, "", copyOpts)
			if err != nil {
				return fmt.Errorf("failed to copy: %w", err)
			}
			err = annotateImage(ctx, dst, manifestDesc, pullInfo.registryOverrideRef, pullInfo.image.Reference)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	return imagesWithManifests, nil
}

func orasSave(ctx context.Context, imagesInfo []imagePullInfo, cfg PullConfig, dst *oci.Store, client *auth.Client) error {
	l := logger.From(ctx)
	for _, imageInfo := range imagesInfo {
		var pullSrc oras.ReadOnlyTarget
		var err error
		remoteRepo := &orasRemote.Repository{PlainHTTP: cfg.PlainHTTP}
		remoteRepo.Reference, err = registry.ParseReference(imageInfo.registryOverrideRef)
		if err != nil {
			return fmt.Errorf("failed to parse image reference %s: %w", imageInfo.registryOverrideRef, err)
		}
		remoteRepo.Client = client

		copyOpts := oras.DefaultCopyOptions
		copyOpts.Concurrency = cfg.Concurrency
		copyOpts.WithTargetPlatform(imageInfo.manifestDesc.Platform)
		l.Info("saving image", "name", imageInfo.registryOverrideRef, "size", utils.ByteFormat(float64(imageInfo.byteSize), 2))
		localCache, err := oci.NewWithContext(ctx, cfg.CacheDirectory)
		if err != nil {
			return fmt.Errorf("failed to create oci formatted directory: %w", err)
		}
		pullSrc = orasCache.New(remoteRepo, localCache)
		desc, err := oras.Copy(ctx, pullSrc, imageInfo.registryOverrideRef, dst, "", copyOpts)
		if err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
		err = annotateImage(ctx, dst, desc, imageInfo.registryOverrideRef, imageInfo.ref)
		if err != nil {
			return err
		}
	}
	return nil
}

func annotateImage(ctx context.Context, dst *oci.Store, desc ocispec.Descriptor, oldRef string, newRef string) error {
	if desc.Annotations == nil {
		desc.Annotations = make(map[string]string)
	}
	desc.Annotations[ocispec.AnnotationRefName] = newRef
	desc.Annotations[ocispec.AnnotationBaseImageName] = newRef
	err := dst.Untag(ctx, oldRef)
	if err != nil {
		return fmt.Errorf("failed to untag image: %w", err)
	}
	err = dst.Tag(ctx, desc, newRef)
	if err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}
	return nil
}
