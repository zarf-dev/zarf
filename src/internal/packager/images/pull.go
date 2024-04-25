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
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

// ImgInfo wraps references/information about an image
type ImgInfo struct {
	RefInfo transform.Image
	Img     v1.Image
}

// PullAll pulls all of the images in the provided tag map.
func (i *ImageConfig) PullAll(ctx context.Context, cancel context.CancelFunc, dst layout.Images) (list []ImgInfo, err error) {

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

	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(10)

	var mu sync.Mutex
	totalBytes := int64(0)
	processing := make(map[string]bool)
	opts := append(config.GetCraneOptions(i.Insecure, i.Architectures...), crane.WithContext(ctx))

	// retry := func(cb func() error) func() error {
	// 	return func() error {
	// 		return helpers.Retry(cb, 3, 5*time.Second, message.Warnf)
	// 	}
	// }

	for idx, refInfo := range i.ImageList {
		refInfo, idx := refInfo, idx
		eg.Go(func() error {
			spinner.Updatef("Fetching image info (%d of %d)", idx+1, len(i.ImageList))

			ref := refInfo.Reference
			for k, v := range i.RegistryOverrides {
				if strings.HasPrefix(refInfo.Reference, k) {
					ref = strings.Replace(refInfo.Reference, k, v, 1)
				}
			}

			var img v1.Image

			// load from local fs if it's a tarball
			if strings.HasSuffix(ref, ".tar") || strings.HasSuffix(ref, ".tar.gz") || strings.HasSuffix(ref, ".tgz") {
				img, err = crane.Load(ref, opts...)
				if err != nil {
					return fmt.Errorf("unable to load %s: %w", refInfo.Reference, err)
				}
			} else {
				reference, err := name.ParseReference(ref)
				if err != nil {
					return fmt.Errorf("failed to parse reference: %w", err)
				}
				_, err = crane.Head(ref, opts...)
				if err != nil {
					if strings.Contains(err.Error(), "unexpected status code 429 Too Many Requests") {
						cancel()
						return fmt.Errorf("rate limited by registry: %w", err)
					}

					message.Notef("Falling back to local 'docker', failed to find the manifest on a remote: %s", err.Error())

					// Attempt to connect to the local docker daemon.
					cli, err := client.NewClientWithOpts(client.FromEnv)
					if err != nil {
						return fmt.Errorf("docker not available: %w", err)
					}
					cli.NegotiateAPIVersion(ctx)

					// Inspect the image to get the size.
					rawImg, _, err := cli.ImageInspectWithRaw(ctx, ref)
					if err != nil {
						if errdefs.IsNotFound(err) {
							cancel()
						}

						return fmt.Errorf("failed to inspect via docker: %w", err)
					}

					// Warn the user if the image is large.
					if rawImg.Size > 750*1000*1000 {
						message.Warnf("%s is %s and may take a very long time to load via docker. "+
							"See https://docs.zarf.dev/faq for suggestions on how to improve large local image loading operations.",
							ref, utils.ByteFormat(float64(rawImg.Size), 2))
					}

					// Use unbuffered opener to avoid OOM Kill issues https://github.com/defenseunicorns/zarf/issues/1214.
					// This will also take forever to load large images.
					img, err = daemon.Image(reference, daemon.WithUnbufferedOpener(), daemon.WithContext(ctx))
					if err != nil {
						return fmt.Errorf("failed to load from docker daemon: %w", err)
					}
				} else {
					img, err = crane.Pull(ref, opts...)
					if err != nil {
						return fmt.Errorf("unable to pull image %s: %w", refInfo.Reference, err)
					}
				}
			}

			img = cache.Image(img, cache.NewFilesystemCache(filepath.Join(config.GetAbsCachePath(), layout.ImagesDir)))

			manifest, err := img.Manifest()
			if err != nil {
				return fmt.Errorf("unable to get manifest for %s: %w", refInfo.Reference, err)
			}
			totalBytes += manifest.Config.Size

			layers, err := img.Layers()
			if err != nil {
				return fmt.Errorf("unable to get layers for %s: %w", refInfo.Reference, err)
			}

			for _, layer := range layers {
				digest, err := layer.Digest()
				if err != nil {
					return fmt.Errorf("unable to get digest for image layer: %w", err)
				}

				if _, ok := processing[digest.Hex]; !ok {
					mu.Lock()
					processing[digest.Hex] = true
					mu.Unlock()
					size, err := layer.Size()
					if err != nil {
						return fmt.Errorf("unable to get size for image layer: %w", err)
					}
					totalBytes += size
				}
			}

			list = append(list, ImgInfo{RefInfo: refInfo, Img: img})
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	clear(processing)

	spinner.Successf("Fetched info for %d images", imageCount)

	doneSaving := make(chan error)
	updateText := fmt.Sprintf("Pulling %d images", imageCount)
	go utils.RenderProgressBarForLocalDirWrite(dst.Base, totalBytes, doneSaving, updateText, updateText)

	eg, _ = errgroup.WithContext(ctx)
	eg.SetLimit(10)

	layerInProgress := errors.New("layer already inprogress")

	markAsProcessing := func(layers []v1.Layer) error {
		mu.Lock()
		defer mu.Unlock()
		for _, layer := range layers {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}

			if _, ok := processing[digest.Hex]; ok {
				message.Debug(helpers.Truncate(digest.Hex, 12, false), "in progress, skipping.")
				return layerInProgress
			}
			processing[digest.Hex] = true
		}
		return nil
	}

	unmarkAsProcessing := func(layers []v1.Layer) error {
		mu.Lock()
		defer mu.Unlock()
		for _, layer := range layers {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}
			delete(processing, digest.Hex)
		}
		return nil
	}

	for _, info := range list {
		refInfo, img := info.RefInfo, info.Img
		eg.Go(func() error {

			layers, err := img.Layers()
			if err != nil {
				return fmt.Errorf("unable to get layers for %s: %w", refInfo.Reference, err)
			}

			for {
				if err := markAsProcessing(layers); err != nil {
					if errors.Is(err, layerInProgress) {
						// message.Debug(processing)
						// time.Sleep(1 * time.Second)
						continue
					}
					return err
				}
				break
			}

			message.Debugf("Pulling image %s", refInfo.Reference)
			annotations := map[string]string{
				ocispec.AnnotationBaseImageName: refInfo.Reference,
			}

			// also have clayout.WithPlatform() as a future option to use
			if err := cranePath.AppendImage(img, clayout.WithAnnotations(annotations)); err != nil {
				return fmt.Errorf("error when trying to save %s: %w", refInfo.Reference, err)
			}

			return unmarkAsProcessing(layers)
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Send a signal to the progress bar that we're done and wait for the thread to finish
	doneSaving <- nil
	<-doneSaving

	return list, nil
}
