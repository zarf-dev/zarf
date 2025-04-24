// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/context/docker"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	cranev1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	clayout "github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"

	"github.com/defenseunicorns/pkg/helpers/v2"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/dns"
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
	cfg.ImageList = helpers.Unique(cfg.ImageList)
	l := logger.From(ctx)
	pullStart := time.Now()

	imageCount := len(cfg.ImageList)
	if err := helpers.CreateDirectory(cfg.DestinationDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", cfg.DestinationDirectory, err)
	}

	if err := helpers.CreateDirectory(cfg.CacheDirectory, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", cfg.DestinationDirectory, err)
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

	// Enable / Disable TLS verification based on the config
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.InsecureSkipTLSVerify}
	orasTransport := retry.NewTransport(transport)
	client.Client.Transport = orasTransport

	l.Debug("gathering credentials from default Docker config file", "credentials_configured", credStore.IsAuthConfigured())
	platform := &ocispec.Platform{
		Architecture: cfg.Arch,
		// TODO: in the future we could support Windows images
		OS: "linux",
	}
	imagesWithManifests := map[transform.Image]ocispec.Manifest{}
	imagesInfo := []imagePullInfo{}
	dockerFallBackImages := []imageDaemonPullInfo{}
	var imageListLock sync.Mutex

	// This loop pulls the metadata from images with three goals
	// - Get all the manifests from images that will be pulled so they can be returned to the function
	// - discover if any images are sha'd to an index, if so error and inform user on the different available platforms
	// - Mark any images that don't resolve so we can attempt to pull them from the daemon
	eg, ectx := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for _, image := range cfg.ImageList {
		image := image
		eg.Go(func() error {
			repo := &orasRemote.Repository{}

			overriddenRef := image.Reference
			for k, v := range cfg.RegistryOverrides {
				if strings.HasPrefix(image.Reference, k) {
					overriddenRef = strings.Replace(image.Reference, k, v, 1)
				}
			}

			repo.Reference, err = registry.ParseReference(overriddenRef)
			if err != nil {
				return err
			}
			repo.Client = client

			repo.PlainHTTP = cfg.PlainHTTP
			if dns.IsLocalhost(repo.Reference.Host()) && !cfg.PlainHTTP {
				repo.PlainHTTP, err = shouldUsePlainHTTP(ctx, repo.Reference.Host(), client)
				// If the pings to localhost fail, it could be an image on the daemon
				if err != nil {
					l.Warn("unable to authenticate to host, attempting pull from docker daemon as fallback", "image", overriddenRef, "err", err)
					imageListLock.Lock()
					defer imageListLock.Unlock()
					dockerFallBackImages = append(dockerFallBackImages, imageDaemonPullInfo{
						image:               image,
						registryOverrideRef: overriddenRef,
					})
					return nil
				}
			}

			fetchOpts := oras.DefaultFetchBytesOptions
			desc, b, err := oras.FetchBytes(ectx, repo, overriddenRef, fetchOpts)
			if err != nil {
				// TODO we could use the k8s library for backoffs here - https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/util/wait/backoff.go
				if strings.Contains(err.Error(), "toomanyrequests") {
					return fmt.Errorf("rate limited by registry: %w", err)
				}
				l.Warn("unable to find image, attempting pull from docker daemon as fallback", "image", overriddenRef, "err", err)
				imageListLock.Lock()
				defer imageListLock.Unlock()
				dockerFallBackImages = append(dockerFallBackImages, imageDaemonPullInfo{
					image:               image,
					registryOverrideRef: overriddenRef,
				})
				return nil
			}

			// If the image sha points to an index then error
			if image.Digest != "" && isIndex(desc.MediaType) {
				// Both index types can be marshalled into an ocispec.Index
				// https://github.com/oras-project/oras-go/blob/853e0125ccad32ff691e4ed70e156c7619021bfd/internal/manifestutil/parser.go#L55
				var idx ocispec.Index
				if err := json.Unmarshal(b, &idx); err != nil {
					return fmt.Errorf("unable to unmarshal index.json: %w", err)
				}
				return constructIndexError(idx, image)
			}
			// If a manifest was returned from FetchBytes, either it's a tag with only one image or it's a non container image
			// If it's not a manifest then we received an index and need to pull the manifest by platform
			if !isManifest(desc.MediaType) {
				fetchOpts.FetchOptions.TargetPlatform = platform
				desc, b, err = oras.FetchBytes(ectx, repo, overriddenRef, fetchOpts)
				if err != nil {
					return fmt.Errorf("failed to fetch image with architecture %s: %w", platform.Architecture, err)
				}
			}

			// extra validation before we marshall, this should never be true
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
			imageListLock.Lock()
			defer imageListLock.Unlock()
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
		return nil, fmt.Errorf("failed to create oci layout: %w", err)
	}

	if len(dockerFallBackImages) > 0 {
		daemonImagesWithManifests, err := pullFromDockerDaemon(ctx, dockerFallBackImages, dst, cfg.Arch, cfg.OCIConcurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to pull images from docker: %w", err)
		}
		maps.Copy(imagesWithManifests, daemonImagesWithManifests)
	}

	err = orasSave(ctx, imagesInfo, cfg, dst, client)
	if err != nil {
		return nil, fmt.Errorf("failed to save images: %w", err)
	}

	l.Debug("done pulling images", "count", len(cfg.ImageList), "duration", time.Since(pullStart))

	return imagesWithManifests, nil
}

func constructIndexError(idx ocispec.Index, image transform.Image) error {
	lines := []string{"The following images are available in the index:"}
	name := image.Name
	if image.Tag != "" {
		name += ":" + image.Tag
	}
	for _, desc := range idx.Manifests {
		lines = append(lines, fmt.Sprintf("image - %s@%s with platform %s", name, desc.Digest, desc.Platform))
	}
	imageOptions := strings.Join(lines, "\n")
	return fmt.Errorf("%s resolved to an OCI image index which is not supported by Zarf, select a specific platform to use: %s", image.Reference, imageOptions)
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

func pullFromDockerDaemon(ctx context.Context, daemonPullInfo []imageDaemonPullInfo, dst *oci.Store, arch string, concurrency int) (map[transform.Image]ocispec.Manifest, error) {
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
			// Pull the image into a Crane directory as the logic for extracting the earlier Docker formats is quite complex
			// Docker starting saving images to the OCI layout format in Feb 2024 in engine version 25
			// Once we feel the user base has updated we can remove Crane here by pulling from the daemon directly then calling oras.Copy
			tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				return fmt.Errorf("failed to make temp directory: %w", err)
			}
			defer os.RemoveAll(tmpDir)
			reference, err := name.ParseReference(pullInfo.registryOverrideRef)
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
				ocispec.AnnotationBaseImageName: pullInfo.image.Reference,
				ocispec.AnnotationRefName:       pullInfo.image.Reference,
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
			desc, b, err := oras.FetchBytes(ctx, dockerImageSrc, pullInfo.image.Reference, fetchBytesOpts)
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
			copyOpts.Concurrency = concurrency
			_, err = oras.Copy(ctx, dockerImageSrc, pullInfo.image.Reference, dst, "", copyOpts)
			if err != nil {
				return fmt.Errorf("failed to copy: %w", err)
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
		repo := &orasRemote.Repository{}
		repo.Reference, err = registry.ParseReference(imageInfo.registryOverrideRef)
		if err != nil {
			return fmt.Errorf("failed to parse image reference %s: %w", imageInfo.registryOverrideRef, err)
		}
		repo.PlainHTTP = cfg.PlainHTTP
		if dns.IsLocalhost(repo.Reference.Host()) && !cfg.PlainHTTP {
			repo.PlainHTTP, err = shouldUsePlainHTTP(ctx, repo.Reference.Host(), client)
			if err != nil {
				return fmt.Errorf("unable to connect to the registry %s: %w", repo.Reference.Host(), err)
			}
		}
		repo.Client = client

		copyOpts := oras.DefaultCopyOptions
		copyOpts.Concurrency = cfg.OCIConcurrency
		copyOpts.WithTargetPlatform(imageInfo.manifestDesc.Platform)
		l.Info("saving image", "name", imageInfo.registryOverrideRef, "size", utils.ByteFormat(float64(imageInfo.byteSize), 2))
		localCache, err := oci.NewWithContext(ctx, cfg.CacheDirectory)
		if err != nil {
			return fmt.Errorf("failed to create oci formatted directory: %w", err)
		}
		pullSrc = orasCache.New(repo, localCache)
		desc, err := oras.Copy(ctx, pullSrc, imageInfo.registryOverrideRef, dst, imageInfo.ref, copyOpts)
		if err != nil {
			return fmt.Errorf("failed to copy: %w", err)
		}
		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string)
		}
		desc.Annotations[ocispec.AnnotationRefName] = imageInfo.ref
		desc.Annotations[ocispec.AnnotationBaseImageName] = imageInfo.ref
		err = dst.Tag(ctx, desc, imageInfo.ref)
		if err != nil {
			return fmt.Errorf("failed to tag image: %w", err)
		}
	}
	return nil
}
