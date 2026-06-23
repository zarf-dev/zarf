// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	"github.com/google/go-containerregistry/pkg/name"
	cranev1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	clayout "github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/versions"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/defenseunicorns/pkg/helpers/v2"
	orasCache "github.com/defenseunicorns/pkg/oci/cache"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/feature"
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
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
	ResponseHeaderTimeout time.Duration
}

type imagePullInfo struct {
	registryOverrideRef string
	ref                 string
	manifestDesc        ocispec.Descriptor
	byteSize            int64
	// platforms is populated only when the image resolves to an OCI image index; one entry per
	// leaf manifest in "arch[/variant]" form. Empty for single-platform manifests.
	platforms []string
}

type imageWithOverride struct {
	overridden transform.Image
	original   transform.Image
}

// Pull pulls all images to the destination directory.
func Pull(ctx context.Context, imageList []transform.Image, destinationDirectory string, opts PullOptions) ([]PulledImage, error) {
	if len(imageList) == 0 {
		return nil, fmt.Errorf("image list is required")
	}
	if destinationDirectory == "" {
		return nil, fmt.Errorf("destination directory is required")
	}
	imageList = helpers.Unique(imageList)
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
	for _, img := range imageList {
		overriddenImage := img
		for _, v := range opts.RegistryOverrides {
			if strings.HasPrefix(img.Reference, v.Source) {
				// If we have an override, the first override wins.
				// Doing so allows earlier, longer prefixes (such as docker.io/library)
				// to supersede shorter prefixes (such as docker.io).
				overriddenImage.Reference = strings.Replace(img.Reference, v.Source, v.Override, 1)
				break
			}
		}
		imagesWithOverride = append(imagesWithOverride, imageWithOverride{
			original:   img,
			overridden: overriddenImage,
		})
	}

	imageFetchStart := time.Now()
	l.Info("fetching info for images", "count", imageCount, "destination", destinationDirectory)

	uniqueHosts := map[string]struct{}{}
	for _, v := range imagesWithOverride {
		uniqueHosts[v.overridden.Host] = struct{}{}
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
		eg.Go(func() error {
			repo := &orasRemote.Repository{}

			ref, err := registry.ParseReference(image.overridden.Reference)
			if err != nil {
				return err
			}
			repo.Reference = ref
			repo.Client = client

			repo.PlainHTTP = opts.PlainHTTP
			if dns.IsLocalhost(repo.Reference.Host()) && !opts.PlainHTTP {
				repo.PlainHTTP, err = ShouldUsePlainHTTP(ctx, repo.Reference.Host(), client)
				// If the pings to localhost fail, it could be an image on the daemon
				if err != nil {
					l.Warn("unable to authenticate to host, attempting pull from docker daemon as fallback", "image", image.overridden.Reference, "err", err)
					imageListLock.Lock()
					defer imageListLock.Unlock()
					dockerFallBackImages = append(dockerFallBackImages, image)
					return nil
				}
			}

			fetchOpts := oras.DefaultFetchBytesOptions
			desc, b, err := oras.FetchBytes(ectx, repo, image.overridden.Reference, fetchOpts)
			if err != nil {
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

			isIndexSha := image.original.Digest != "" && IsIndex(desc.MediaType)
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
				registryOverrideRef: image.overridden.Reference,
				ref:                 image.original.Reference,
				byteSize:            size,
				manifestDesc:        desc,
				platforms:           platforms,
			})
			pulledImages = append(pulledImages, PulledImage{Image: image.original})
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

	if len(dockerFallBackImages) > 0 {
		daemonImagesWithManifests, err := pullFromDockerDaemon(ctx, dockerFallBackImages, dst, opts.Arch, opts.OCIConcurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to pull images from docker: %w", err)
		}
		pulledImages = append(pulledImages, daemonImagesWithManifests...)
	}

	for _, imageInfo := range imagesInfo {
		err = orasSave(ctx, imageInfo, opts, dst, client)
		if err != nil {
			return nil, fmt.Errorf("failed to save images: %w", err)
		}
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

func pullFromDockerDaemon(ctx context.Context, daemonImages []imageWithOverride, dst *oci.Store, arch string, concurrency int) (_ []PulledImage, err error) {
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
	// Saving images directly from the Docker daemon's OCI image export is faster and simpler than Crane, but it
	// requires Docker engine v25.0+ (Jan 2024), the first version to export the OCI layout format. We only take the
	// direct path when the feature is enabled (the default) and the daemon is new enough; otherwise we fall back to
	// Crane, which handles the older export formats. The feature flag can be disabled to force the Crane path.
	directPull := feature.IsEnabled(feature.DockerDaemonDirectPull) && daemonSupportsOCIExport(ctx, cli)
	for _, daemonImage := range daemonImages {
		var pullErr error
		if directPull {
			pullErr = saveImageFromDockerDaemon(ctx, cli, dst, daemonImage, arch, concurrency)
		} else {
			pullErr = craneSaveImageFromDockerDaemon(ctx, cli, dst, daemonImage, arch, concurrency)
		}
		if pullErr != nil {
			return nil, pullErr
		}
		pulledImages = append(pulledImages, PulledImage{Image: daemonImage.original})
	}

	return pulledImages, nil
}

// minDockerVersionForOCIExport is the first Docker engine version (released Jan 2024) to export images in the OCI
// layout format via ImageSave. Older engines use legacy formats that require the Crane-based fallback.
var minDockerVersionForOCIExport = semver.MustParse("25.0.0")

// daemonSupportsOCIExport reports whether the connected Docker daemon is new enough to export images in the OCI
// layout format. If the version can't be determined or parsed, it returns false so the caller falls back to Crane.
func daemonSupportsOCIExport(ctx context.Context, cli *client.Client) bool {
	l := logger.From(ctx)
	v, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		l.Debug("unable to determine docker daemon version, using crane for daemon pull", "err", err)
		return false
	}
	ver, err := semver.NewVersion(v.Version)
	if err != nil {
		l.Debug("unable to parse docker daemon version, using crane for daemon pull", "version", v.Version, "err", err)
		return false
	}
	return !ver.LessThan(minDockerVersionForOCIExport)
}

// saveImageFromDockerDaemon exports a single image from the Docker daemon via the engine's OCI image export
// (the equivalent of `docker save`) and copies it into dst. This requires Docker engine v25.0+ (Feb 2024), the
// first version to export images in the OCI layout format.
func saveImageFromDockerDaemon(ctx context.Context, cli *client.Client, dst *oci.Store, daemonImage imageWithOverride, arch string, concurrency int) (err error) {
	l := logger.From(ctx)
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

	imageTarPath := filepath.Join(tmpDir, "image.tar")
	tarFile, err := os.Create(imageTarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar file: %w", err)
	}
	if _, err := io.Copy(tarFile, imageReader); err != nil {
		return errors.Join(fmt.Errorf("failed to write image to tar file: %w", err), tarFile.Close())
	}
	if err := tarFile.Close(); err != nil {
		return fmt.Errorf("failed to close tar file: %w", err)
	}

	dockerImageOCILayoutPath := filepath.Join(tmpDir, "docker-image-oci-layout")
	if err := archive.Decompress(ctx, imageTarPath, dockerImageOCILayoutPath, archive.DecompressOpts{}); err != nil {
		return fmt.Errorf("failed to extract image tar: %w", err)
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
	l.Info("pulling image from docker daemon", "name", daemonImage.overridden.Reference)
	if _, err := copyImageFromOCILayout(ctx, dockerImageSrc, dst, manifests[0].Digest.String(), daemonImage.original, arch, concurrency); err != nil {
		return err
	}
	return nil
}

// craneSaveImageFromDockerDaemon exports a single image from the Docker daemon using Crane and copies it into dst.
// Crane handles the older, pre-OCI-layout Docker export formats whose extraction logic is quite complex. Once the
// user base has moved to Docker engine v25.0+ this can be removed in favor of saveImageFromDockerDaemon.
func craneSaveImageFromDockerDaemon(ctx context.Context, cli *client.Client, dst *oci.Store, daemonImage imageWithOverride, arch string, concurrency int) (err error) {
	l := logger.From(ctx)
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fmt.Errorf("failed to make temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()
	reference, err := name.ParseReference(daemonImage.overridden.Reference)
	if err != nil {
		return fmt.Errorf("failed to parse reference: %w", err)
	}
	// Use unbuffered opener to avoid OOM Kill issues https://github.com/zarf-dev/zarf/issues/1214.
	// This will also take forever to load large images.
	img, err := daemon.Image(reference, daemon.WithUnbufferedOpener(), daemon.WithClient(cli))
	if err != nil {
		return fmt.Errorf("failed to load from docker daemon: %w", err)
	}
	cranePath, err := clayout.Write(tmpDir, empty.Index)
	if err != nil {
		return fmt.Errorf("failed to create OCI layout: %w", err)
	}
	if err := cranePath.WriteImage(img); err != nil {
		return fmt.Errorf("failed to write docker image: %w", err)
	}
	annotations := map[string]string{
		ocispec.AnnotationBaseImageName: daemonImage.original.Reference,
		ocispec.AnnotationRefName:       daemonImage.original.Reference,
	}
	platform := &ocispec.Platform{
		Architecture: arch,
		OS:           "linux",
	}
	cranePlatform := cranev1.Platform{
		OS:           platform.OS,
		Architecture: platform.Architecture,
	}
	err = cranePath.AppendImage(img, clayout.WithAnnotations(annotations), clayout.WithPlatform(cranePlatform))
	if err != nil {
		return fmt.Errorf("failed to write image: %w", err)
	}

	// Needed because when pulling from the local docker daemon, while using the docker containerd runtime
	// Crane incorrectly names the blob of the docker image config to a sha that does not match the contents
	// https://github.com/zarf-dev/zarf/issues/2584
	// This is a band aid fix while we wait for crane and or docker to create the permanent fix
	blobDir := filepath.Join(tmpDir, "blobs", "sha256")
	err = filepath.Walk(blobDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		hash, err := helpers.GetSHA256OfFile(path)
		if err != nil {
			return err
		}
		newFile := filepath.Join(blobDir, hash)
		return os.Rename(path, newFile)
	})
	if err != nil {
		return err
	}

	dockerImageSrc, err := oci.NewWithContext(ctx, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to create OCI store: %w", err)
	}
	fetchBytesOpts := oras.DefaultFetchBytesOptions
	fetchBytesOpts.TargetPlatform = platform
	desc, b, err := oras.FetchBytes(ctx, dockerImageSrc, daemonImage.original.Reference, fetchBytesOpts)
	if err != nil {
		return fmt.Errorf("failed to get manifest from docker image source: %w", err)
	}
	if !IsManifest(desc.MediaType) {
		return fmt.Errorf("expected to find image manifest instead found %s", desc.MediaType)
	}
	size, err := getSizeOfManifest(desc, b)
	if err != nil {
		return err
	}
	l.Info("pulling image from docker daemon", "name", daemonImage.overridden.Reference, "size", utils.ByteFormat(float64(size), 2))
	copyOpts := oras.DefaultCopyOptions
	copyOpts.WithTargetPlatform(platform)
	copyOpts.Concurrency = concurrency
	_, err = oras.Copy(ctx, dockerImageSrc, daemonImage.original.Reference, dst, "", copyOpts)
	if err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}
	return nil
}

func orasSave(ctx context.Context, imageInfo imagePullInfo, opts PullOptions, dst *oci.Store, client *auth.Client) error {
	l := logger.From(ctx)
	var pullSrc oras.ReadOnlyTarget
	var err error
	repo := &orasRemote.Repository{}
	repo.Reference, err = registry.ParseReference(imageInfo.registryOverrideRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference %s: %w", imageInfo.registryOverrideRef, err)
	}
	repo.PlainHTTP = opts.PlainHTTP
	if dns.IsLocalhost(repo.Reference.Host()) && !opts.PlainHTTP {
		repo.PlainHTTP, err = ShouldUsePlainHTTP(ctx, repo.Reference.Host(), client)
		if err != nil {
			return fmt.Errorf("unable to connect to the registry %s: %w", repo.Reference.Host(), err)
		}
	}
	repo.Client = client

	copyOpts := oras.DefaultCopyOptions
	copyOpts.Concurrency = opts.OCIConcurrency
	copyOpts.WithTargetPlatform(imageInfo.manifestDesc.Platform)
	logArgs := []any{"name", imageInfo.registryOverrideRef, "size", utils.ByteFormat(float64(imageInfo.byteSize), 2)}
	if len(imageInfo.platforms) > 0 {
		logArgs = append(logArgs, "platforms", strings.Join(imageInfo.platforms, ","))
	}
	l.Info("saving image", logArgs...)
	localCache, err := oci.NewWithContext(ctx, opts.CacheDirectory)
	if err != nil {
		return fmt.Errorf("failed to create oci formatted directory: %w", err)
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
