// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	retry "github.com/avast/retry-go/v4"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/flags"
	"github.com/mholt/archives"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	orasCache "github.com/zarf-dev/zarf/src/pkg/oci/cache"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// PullOptions is the configuration for pulling images.
type PullOptions struct {
	OCIConcurrency        int
	Arch                  string
	RegistryOverrides     []RegistryOverride
	CacheDirectory        string
	InsecureSkipTLSVerify bool
	ResponseHeaderTimeout time.Duration
	// PlainHTTP is not applied directly to these (third-party) image references — it
	// gates whether transport is negotiated per host at all.
	PlainHTTP bool
}

// ImageRequest pairs an image with its source. An empty Source means registry.
type ImageRequest struct {
	Image  transform.Image
	Source api.ImageSource
}

type imagePullInfo struct {
	request             ImageRequest
	registryOverrideRef string
	ref                 string
	manifestDesc        ocispec.Descriptor
	byteSize            int64
	// platforms is populated only when the image resolves to an OCI image index; one entry per
	// leaf manifest in "arch[/variant]" form. Empty for single-platform manifests.
	platforms []string
	// plainHTTP is the transport scheme negotiated for this image's registry during the
	// metadata-fetch pass
	plainHTTP bool
}

type imageWithOverride struct {
	overridden transform.Image
	request    ImageRequest
}

// Pull pulls all images to the destination directory.
func Pull(ctx context.Context, imageList []ImageRequest, destinationDirectory string, opts PullOptions) ([]PulledImage, error) {
	if len(imageList) == 0 {
		return nil, fmt.Errorf("image list is required")
	}
	if destinationDirectory == "" {
		return nil, fmt.Errorf("destination directory is required")
	}
	uniqueImages := make([]ImageRequest, 0, len(imageList))
	seenSources := make(map[string]api.ImageSource, len(imageList))
	for _, request := range imageList {
		request.Source = request.Source.GetSource()
		if previous, exists := seenSources[request.Image.Reference]; exists {
			if previous != request.Source {
				return nil, fmt.Errorf("image %q has conflicting sources %q and %q", request.Image.Reference, previous, request.Source)
			}
			continue
		}
		seenSources[request.Image.Reference] = request.Source
		uniqueImages = append(uniqueImages, request)
	}
	imageList = uniqueImages
	l := logger.From(ctx)
	pullStart := time.Now()

	imageCount := len(imageList)
	if err := helpers.CreateDirectory(destinationDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", destinationDirectory, err)
	}

	if err := helpers.CreateDirectory(opts.CacheDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", destinationDirectory, err)
	}

	if opts.ResponseHeaderTimeout < 0 {
		opts.ResponseHeaderTimeout = 0 // currently allowing infinite timeout
	}

	imagesWithOverride := []imageWithOverride{}
	// Iterate over all images, marking each one as overridden.
	for _, request := range imageList {
		overriddenImage := request.Image
		if request.Source != api.ImageSourceDaemon {
			for _, v := range opts.RegistryOverrides {
				if strings.HasPrefix(request.Image.Reference, v.Source) {
					// If we have an override, the first override wins.
					// Doing so allows earlier, longer prefixes (such as docker.io/library)
					// to supersede shorter prefixes (such as docker.io).
					overriddenImage.Reference = strings.Replace(request.Image.Reference, v.Source, v.Override, 1)
					break
				}
			}
		}
		imagesWithOverride = append(imagesWithOverride, imageWithOverride{
			overridden: overriddenImage,
			request:    request,
		})
	}

	imageFetchStart := time.Now()
	l.Info("fetching info for images", "count", imageCount, "destination", destinationDirectory)

	uniqueHosts := map[string]struct{}{}
	for _, v := range imagesWithOverride {
		if v.request.Source != api.ImageSourceDaemon {
			ref, err := registry.ParseReference(v.overridden.Reference)
			if err != nil {
				return nil, fmt.Errorf("invalid image reference %q: %w", v.overridden.Reference, err)
			}
			uniqueHosts[ref.Host()] = struct{}{}
		}
	}
	client, err := NewAuthClientFromDocker(ctx, opts.InsecureSkipTLSVerify, opts.ResponseHeaderTimeout, uniqueHosts)
	if err != nil {
		return nil, err
	}

	platform := &ocispec.Platform{
		Architecture: opts.Arch,
		// TODO: in the future we could support Windows images
		OS: "linux",
	}
	pulledImages := []PulledImage{}
	imagesInfo := []imagePullInfo{}
	dockerFallBackImages := []imageWithOverride{}
	var imageListLock sync.Mutex

	// This loop pulls the metadata from images with three goals
	// - Get all the manifests from images that will be pulled so they can be returned to the function
	// - Mark any images that don't resolve so we can attempt to pull them from the daemon
	eg, ectx := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for _, image := range imagesWithOverride {
		if image.request.Source == api.ImageSourceDaemon {
			imageListLock.Lock()
			dockerFallBackImages = append(dockerFallBackImages, image)
			imageListLock.Unlock()
			continue
		}
		eg.Go(func() error {
			repo := &orasRemote.Repository{}

			ref, err := registry.ParseReference(image.overridden.Reference)
			if err != nil {
				return err
			}
			repo.Reference = ref
			repo.Client = client

			// Only verify per-host when the flag is set, or when the host is
			// localhost: a local dev/test registry is commonly plain HTTP even
			// when --plain-http wasn't passed for that specific purpose.
			var plainHTTP bool
			if opts.PlainHTTP || dns.IsLocalOrPrivate(repo.Reference.Host()) {
				plainHTTP, err = ocischeme.From(ctx).UsePlainHTTP(ctx, repo.Reference.Host(), ocischeme.ProbeOptions{InsecureSkipTLSVerify: opts.InsecureSkipTLSVerify})
				if err != nil {
					if image.request.Source == api.ImageSourceRegistry {
						return fmt.Errorf("unable to reach registry for image %q: %w", image.overridden.Reference, err)
					}
					// It could be an image on the daemon instead of a registry.
					l.Warn("unable to reach registry, attempting pull from docker daemon as fallback", "image", image.overridden.Reference, "err", err)
					imageListLock.Lock()
					defer imageListLock.Unlock()
					dockerFallBackImages = append(dockerFallBackImages, image)
					return nil
				}
			}
			repo.PlainHTTP = plainHTTP

			fetchOpts := oras.DefaultFetchBytesOptions
			desc, b, err := oras.FetchBytes(ectx, repo, image.overridden.Reference, fetchOpts)
			if err != nil {
				if image.request.Source == api.ImageSourceRegistry {
					return fmt.Errorf("unable to fetch registry image %q: %w", image.overridden.Reference, err)
				}
				// TODO we could use the k8s library for backoffs here - https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/util/wait/backoff.go
				if strings.Contains(err.Error(), "toomanyrequests") {
					return fmt.Errorf("rate limited by registry: %w", err)
				}
				l.Warn("unable to find image, attempting pull from docker daemon as fallback", "image", image.overridden.Reference, "err", err)
				imageListLock.Lock()
				defer imageListLock.Unlock()
				dockerFallBackImages = append(dockerFallBackImages, image)
				return nil
			}

			isIndexSha := image.request.Image.Digest != "" && IsIndex(desc.MediaType)
			// If a manifest was returned from FetchBytes, either it's a tag with only one image or it's a non container image
			// If it's not a manifest then we received an index and need to pull the manifest by platform
			if !IsManifest(desc.MediaType) && !isIndexSha {
				fetchOpts.FetchOptions.TargetPlatform = platform
				desc, b, err = oras.FetchBytes(ectx, repo, image.overridden.Reference, fetchOpts)
				if err != nil {
					return fmt.Errorf("failed to fetch image %s with architecture %s: %w", image.overridden.Reference, platform.Architecture, err)
				}
			}

			var size int64
			var platforms []string
			switch {
			case IsIndex(desc.MediaType):
				size, platforms, err = inspectIndex(ectx, repo, desc, b)
				if err != nil {
					return fmt.Errorf("failed to inspect index %s: %w", image.overridden.Reference, err)
				}
			case IsManifest(desc.MediaType):
				size, err = getSizeOfManifest(desc, b)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("received unexpected mediatype %s", desc.MediaType)
			}
			imageListLock.Lock()
			defer imageListLock.Unlock()
			imagesInfo = append(imagesInfo, imagePullInfo{
				request:             image.request,
				registryOverrideRef: image.overridden.Reference,
				ref:                 image.request.Image.Reference,
				byteSize:            size,
				manifestDesc:        desc,
				platforms:           platforms,
				plainHTTP:           plainHTTP,
			})
			l.Debug("pulled image", "name", image.overridden.Reference)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	l.Debug("done fetching info for images", "count", imageCount, "duration", time.Since(imageFetchStart))

	l.Info("pulling images", "count", imageCount)

	dst, err := oci.NewWithContext(ctx, destinationDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to create oci layout: %w", err)
	}

	// Daemon fallbacks and registry images are two halves of the same pull, so they share a single
	// index counted against imageCount. Daemon images go first, offsetting the registry images below.
	daemonImageCount := len(dockerFallBackImages)
	if daemonImageCount > 0 {
		daemonImagesWithManifests, err := pullFromDockerDaemon(ctx, dockerFallBackImages, dst, opts.Arch, opts.OCIConcurrency, imageCount)
		if err != nil {
			return nil, fmt.Errorf("failed to pull images from docker: %w", err)
		}
		pulledImages = append(pulledImages, daemonImagesWithManifests...)
	}

	for index, imageInfo := range imagesInfo {
		err = orasSave(ctx, imageInfo, daemonImageCount+index+1, imageCount, opts, dst, client)
		if err != nil {
			return nil, fmt.Errorf("failed to save images: %w", err)
		}
		pulledImages = append(pulledImages, PulledImage{Image: imageInfo.request.Image})
	}

	l.Info("done pulling images", "count", imageCount, "duration", time.Since(pullStart).Round(time.Millisecond*100))

	return pulledImages, nil
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

func pullFromDockerDaemon(ctx context.Context, daemonImages []imageWithOverride, dst *oci.Store, arch string, concurrency, count int) (_ []PulledImage, err error) {
	pulledImages := []PulledImage{}
	dockerEndPointHost, err := getDockerEndpointHost()
	if err != nil {
		return nil, err
	}
	cli, err := client.New(
		client.WithHost(dockerEndPointHost),
		client.WithTLSClientConfigFromEnv(),
		client.WithAPIVersionFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer func() {
		err = errors.Join(err, cli.Close())
	}()
	if err := checkDaemonOCIExportSupport(ctx, cli); err != nil {
		return nil, err
	}
	for index, daemonImage := range daemonImages {
		if err := saveImageFromDockerDaemon(ctx, cli, dst, daemonImage, arch, concurrency, index+1, count); err != nil {
			return nil, err
		}
		pulledImages = append(pulledImages, PulledImage{Image: daemonImage.request.Image})
	}

	return pulledImages, nil
}

// minDockerVersionForOCIExport is the first Docker engine version to export images in OCI layout format via ImageSave.
var minDockerVersionForOCIExport = semver.MustParse("25.0.0")

func checkDaemonOCIExportSupport(ctx context.Context, cli *client.Client) error {
	v, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Docker server version: %w", err)
	}
	ver, err := semver.NewVersion(v.Version)
	if err != nil {
		return fmt.Errorf("failed to parse Docker server version %q: %w", v.Version, err)
	}
	if ver.LessThan(minDockerVersionForOCIExport) {
		return fmt.Errorf("pulling images from the Docker daemon requires Docker >=%s; found %s", minDockerVersionForOCIExport, v.Version)
	}
	return nil
}

// saveImageFromDockerDaemon exports a single image from the Docker daemon via the engine's OCI image export
// (the equivalent of `docker save`) and copies it into dst.
func saveImageFromDockerDaemon(ctx context.Context, cli *client.Client, dst *oci.Store, daemonImage imageWithOverride, arch string, concurrency, index, count int) (err error) {
	l := logger.From(ctx)
	l.Debug("pulling image from the Docker Daemon using Docker SDK")
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fmt.Errorf("failed to make temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	// Passing a platform only has an effect on multi-platform images and requires client API version 1.48 (released
	// Feb 2025); ImageSave errors if we send it to older clients, so we only set it when the negotiated version
	// supports it.
	var saveOpts []client.ImageSaveOption
	if versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.48") {
		saveOpts = append(saveOpts, client.ImageSaveWithPlatforms(ocispec.Platform{Architecture: arch, OS: "linux"}))
	}
	imageReader, err := cli.ImageSave(ctx, []string{daemonImage.overridden.Reference}, saveOpts...)
	if err != nil {
		return fmt.Errorf("failed to save image %s from docker daemon: %w", daemonImage.overridden.Reference, err)
	}
	defer func() {
		err = errors.Join(err, imageReader.Close())
	}()

	// ImageSave returns an uncompressed tar, so we extract the stream straight into an OCI
	// layout rather than materializing the full image as an intermediate file on disk.
	dockerImageOCILayoutPath := filepath.Join(tmpDir, "docker-image-oci-layout")
	if err := archive.DecompressStream(ctx, imageReader, dockerImageOCILayoutPath, archive.DecompressOpts{Extractor: archives.Tar{}}); err != nil {
		return fmt.Errorf("failed to extract image from docker daemon: %w", err)
	}
	manifests, err := getManifestsFromOCILayout(dockerImageOCILayoutPath)
	if err != nil {
		return err
	}
	// The export of a single image should always contain exactly one manifest.
	if len(manifests) != 1 {
		return fmt.Errorf("expected exactly one manifest in image export, found %d", len(manifests))
	}

	dockerImageSrc, err := oci.NewWithContext(ctx, dockerImageOCILayoutPath)
	if err != nil {
		return fmt.Errorf("failed to create OCI store: %w", err)
	}
	l.Info("pulling image from docker daemon", "name", daemonImage.overridden.Reference, "count", fmt.Sprintf("%d/%d", index, count))
	_, err = copyImageFromOCILayout(ctx, dockerImageSrc, dst, manifests[0].Digest.String(), daemonImage.request.Image, arch, concurrency)
	return err
}

func orasSave(ctx context.Context, imageInfo imagePullInfo, index, count int, opts PullOptions, dst *oci.Store, client *auth.Client) error {
	l := logger.From(ctx)
	var pullSrc oras.ReadOnlyTarget
	var err error
	repo := &orasRemote.Repository{}
	repo.Reference, err = registry.ParseReference(imageInfo.registryOverrideRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference %s: %w", imageInfo.registryOverrideRef, err)
	}
	// The transport scheme was already negotiated for this host during the metadata-fetch
	// pass (the host is known-reachable at this point); reuse it rather than re-probing.
	repo.PlainHTTP = imageInfo.plainHTTP
	repo.Client = client

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = opts.OCIConcurrency
	copyOpts.WithTargetPlatform(imageInfo.manifestDesc.Platform)
	logArgs := []any{"name", imageInfo.registryOverrideRef, "size", utils.ByteFormat(float64(imageInfo.byteSize), 2)}
	if len(imageInfo.platforms) > 0 {
		logArgs = append(logArgs, "platforms", strings.Join(imageInfo.platforms, ","))
	}
	logArgs = append(logArgs, "count", fmt.Sprintf("%d/%d", index, count))
	l.Info("saving image", logArgs...)
	localCache, err := oci.NewStorage(opts.CacheDirectory)
	if err != nil {
		return fmt.Errorf("failed to create oci cache storage: %w", err)
	}
	pullSrc = orasCache.New(repo, localCache)
	var desc ocispec.Descriptor
	err = retry.Do(
		func() error {
			trackedDst := NewTrackedTarget(dst, imageInfo.byteSize, DefaultReport(l, "image pull in progress", imageInfo.registryOverrideRef))
			trackedDst.StartReporting(ctx)
			defer trackedDst.StopReporting()
			var copyErr error
			desc, copyErr = oras.Copy(ctx, pullSrc, imageInfo.registryOverrideRef, trackedDst, imageInfo.ref, copyOpts)
			return copyErr
		},
		retry.Attempts(uint(config.ZarfDefaultRetries)),
		retry.Delay(config.ZarfDefaultRetryDelay),
		retry.MaxDelay(config.ZarfDefaultRetryMaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			if config.ZarfDefaultRetries > 1 && n+1 < uint(config.ZarfDefaultRetries) {
				l.Warn("retrying image pull",
					"attempt", n+1,
					"maxAttempts", config.ZarfDefaultRetries,
					"image", imageInfo.registryOverrideRef,
					"error", err,
				)
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}
	desc = addNameAnnotationsToDesc(desc, imageInfo.ref)
	err = dst.Tag(ctx, desc, imageInfo.ref)
	if err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}
	return nil
}
