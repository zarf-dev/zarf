// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/docker/docker/errdefs"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	clayout "github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/moby/moby/client"
	"golang.org/x/sync/errgroup"
)

// ImgInfo wraps references/information about an image
type ImgInfo struct {
	RefInfo transform.Image
	Img     v1.Image
}

// PullAll pulls all of the images in the provided tag map.
func (i *ImageConfig) PullAll(ctx context.Context, dst layout.Images) (list []ImgInfo, err error) {

	var longer string
	imageCount := len(i.ImageList)
	// Give some additional user feedback on larger image sets
	if imageCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imageCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	if err := helpers.CreateDirectory(dst.Base, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", dst.Base, err)
	}

	cranePath, err := clayout.FromPath(dst.Base)
	if err != nil {
		cranePath, err = clayout.Write(dst.Base, empty.Index)
		if err != nil {
			return nil, err
		}
	}

	spinner := message.NewProgressSpinner("Fetching info for %d images. %s", imageCount, longer)
	defer spinner.Stop()

	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	ctx, cancel := context.WithCancel(ctx)
	eg, ectx := errgroup.WithContext(ctx)
	defer cancel()
	eg.SetLimit(10)

	// refInfoToImage := make(map[transform.Image]v1.Image)
	refInfoToImage := make(map[transform.Image]v1.Image)
	var mu sync.Mutex
	totalBytes := int64(0)
	processed := make(map[string]v1.Layer)
	opts := append(config.GetCraneOptions(i.Insecure, i.Architectures...), crane.WithContext(ctx))

	for idx, refInfo := range i.ImageList {
		refInfo, idx := refInfo, idx
		eg.Go(func() error {
			spinner.Updatef("Fetching image info (%d of %d)", idx+1, len(i.ImageList))

			actual := refInfo.Reference
			for k, v := range i.RegistryOverrides {
				if strings.HasPrefix(refInfo.Reference, k) {
					actual = strings.Replace(refInfo.Reference, k, v, 1)
				}
			}

			var img v1.Image

			// load from local fs if it's a tarball
			if strings.HasSuffix(actual, ".tar") || strings.HasSuffix(actual, ".tar.gz") || strings.HasSuffix(actual, ".tgz") {
				img, err = crane.Load(actual, opts...)
				if err != nil {
					return fmt.Errorf("unable to load image %s: %w", refInfo.Reference, err)
				}
			} else {
				_, err = crane.Head(actual, opts...)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return err
					}

					if strings.Contains(err.Error(), "unexpected status code 429 Too Many Requests") {
						cancel()
						return fmt.Errorf("rate limited by registry: %w", err)
					}

					message.Notef("Falling back to local 'docker' images, failed to find the manifest on a remote: %s", err.Error())

					reference, err := name.ParseReference(actual)
					if err != nil {
						return fmt.Errorf("failed to parse image reference: %w", err)
					}

					// Attempt to connect to the local docker daemon.
					cli, err := client.NewClientWithOpts(client.FromEnv)
					if err != nil {
						return fmt.Errorf("docker not available: %w", err)
					}
					cli.NegotiateAPIVersion(ectx)

					// Inspect the image to get the size.
					rawImg, _, err := cli.ImageInspectWithRaw(ectx, actual)
					if err != nil {
						if errdefs.IsNotFound(err) {
							cancel()
						}

						return fmt.Errorf("failed to inspect image via docker: %w", err)
					}

					// Warn the user if the image is large.
					if rawImg.Size > 750*1000*1000 {
						message.Warnf("%s is %s and may take a very long time to load via docker. "+
							"See https://docs.zarf.dev/faq for suggestions on how to improve large local image loading operations.",
							actual, utils.ByteFormat(float64(rawImg.Size), 2))
					}

					// Use unbuffered opener to avoid OOM Kill issues https://github.com/defenseunicorns/zarf/issues/1214.
					// This will also take forever to load large images.
					img, err = daemon.Image(reference, daemon.WithUnbufferedOpener(), daemon.WithContext(ectx))
					if err != nil {
						return fmt.Errorf("failed to load image from docker daemon: %w", err)
					}
				} else {
					img, err = crane.Pull(actual, opts...)
					if err != nil {
						return fmt.Errorf("unable to pull image %s: %w", refInfo.Reference, err)
					}
				}
			}

			img = cache.Image(img, cache.NewFilesystemCache(filepath.Join(config.GetAbsCachePath(), layout.ImagesDir)))

			manifest, err := img.Manifest()
			if err != nil {
				return fmt.Errorf("unable to get manifest for image %s: %w", refInfo.Reference, err)
			}
			totalBytes += manifest.Config.Size

			layers, err := img.Layers()
			if err != nil {
				return fmt.Errorf("unable to get layers for image %s: %w", refInfo.Reference, err)
			}

			for _, layer := range layers {
				digest, err := layer.Digest()
				if err != nil {
					return fmt.Errorf("unable to get digest for image layer: %w", err)
				}

				if _, ok := processed[digest.Hex]; !ok {
					processed[digest.Hex] = layer
					size, err := layer.Size()
					if err != nil {
						return fmt.Errorf("unable to get size for image layer: %w", err)
					}
					totalBytes += size
				}
			}

			mu.Lock()
			refInfoToImage[refInfo] = img
			mu.Unlock()
			list = append(list, ImgInfo{RefInfo: refInfo, Img: img})
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	spinner.Success()
	// Create a thread to update a progress bar as we save the image files to disk
	doneSaving := make(chan error)
	updateText := fmt.Sprintf("Pulling %d images", imageCount)
	go utils.RenderProgressBarForLocalDirWrite(dst.Base, totalBytes, doneSaving, updateText, updateText)

	referenceToDigest := make(map[string]string)

	eg, ectx = errgroup.WithContext(ctx)
	eg.SetLimit(10)

	// Spawn a goroutine for each image to write it's config and manifest to disk using crane
	for refInfo, img := range refInfoToImage {
		// Create a closure so that we can pass the refInfo and img into the goroutine
		refInfo, img := refInfo, img
		eg.Go(func() error {
			if err := cranePath.WriteImage(img); err != nil {
				// Check if the cache has been invalidated, and warn the user if so
				if strings.HasPrefix(err.Error(), "error writing layer: expected blob size") {
					message.Warnf("Potential image cache corruption: %s - try clearing cache with \"zarf tools clear-cache\"", err.Error())
				}
				return fmt.Errorf("error when trying to save the img (%s): %w", refInfo.Reference, err)
			}

			desc, err := partial.Descriptor(img)
			if err != nil {
				return err
			}

			if err := cranePath.AppendDescriptor(*desc); err != nil {
				return err
			}

			imgDigest, err := img.Digest()
			if err != nil {
				return err
			}

			mu.Lock()
			referenceToDigest[refInfo.Reference] = imgDigest.String()
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if err := utils.AddImageNameAnnotation(dst.Base, referenceToDigest); err != nil {
		return nil, fmt.Errorf("unable to format OCI layout: %w", err)
	}

	// Send a signal to the progress bar that we're done and wait for the thread to finish
	doneSaving <- nil
	<-doneSaving

	return list, nil
}
