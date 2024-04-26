// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	shas := make(map[string]bool)
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

			mt, err := img.MediaType()
			if err != nil {
				return fmt.Errorf("unable to get media type for %s: %w", refInfo.Reference, err)
			}

			if refInfo.Digest != "" && mt.IsIndex() {
				message.Warn("Zarf does not currently support direct consumption of OCI image indexes or Docker manifest lists")

				var idx v1.IndexManifest
				b, err := img.RawManifest()
				if err != nil {
					return fmt.Errorf("unable to get raw manifest for %s: %w", refInfo.Reference, err)
				}
				if err := json.Unmarshal(b, &idx); err != nil {
					return fmt.Errorf("unable to unmarshal index manifest: %w", err)
				}
				message.Warn("The following images are available in the index:")
				for _, desc := range idx.Manifests {
					message.Warnf("%s%s for platform %s", refInfo.Name, refInfo.TagOrDigest, desc.Platform)
				}
				cancel()
				return fmt.Errorf("%s resolved to an index, please select a specific platform to use", refInfo.Reference)
			}

			layers, err := img.Layers()
			if err != nil {
				return fmt.Errorf("unable to get layers for %s: %w", refInfo.Reference, err)
			}

			for _, layer := range layers {
				digest, err := layer.Digest()
				if err != nil {
					return fmt.Errorf("unable to get digest for image layer: %w", err)
				}

				if _, ok := shas[digest.Hex]; !ok {
					mu.Lock()
					shas[digest.Hex] = true
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

	clear(shas)

	spinner.Successf("Fetched info for %d images", imageCount)

	doneSaving := make(chan error)
	updateText := fmt.Sprintf("Pulling %d images", imageCount)
	go utils.RenderProgressBarForLocalDirWrite(dst.Base, totalBytes, doneSaving, updateText, updateText)

	toSave := map[string]v1.Image{}
	for _, info := range list {
		toSave[info.RefInfo.Reference] = info.Img
	}

	sc := func() error {
		saved, err := SaveConcurrent(ctx, cranePath, toSave)
		if err != nil {
			return err
		}
		for k := range saved {
			delete(toSave, k)
		}
		return nil
	}

	ss := func() error {
		saved, err := SaveSequential(cranePath, toSave)
		if err != nil {
			return err
		}
		for k := range saved {
			delete(toSave, k)
		}
		return nil
	}

	if err := helpers.Retry(sc, 2, 5*time.Second, message.Warnf); err != nil {
		message.Warnf("Failed to save images in parallel, falling back to sequential save: %s", err.Error())

		if err := helpers.Retry(ss, 2, 5*time.Second, message.Warnf); err != nil {
			return nil, err
		}
	}

	// Send a signal to the progress bar that we're done and wait for the thread to finish
	doneSaving <- nil
	<-doneSaving

	return list, nil
}

// SaveSequential saves images sequentially.
func SaveSequential(cl clayout.Path, m map[string]v1.Image) (map[string]v1.Image, error) {
	saved := map[string]v1.Image{}
	for name, img := range m {
		name, img := name, img
		annotations := map[string]string{
			ocispec.AnnotationBaseImageName: name,
		}
		if err := cl.AppendImage(img, clayout.WithAnnotations(annotations)); err != nil {
			return saved, fmt.Errorf("error when trying to save %s: %w", name, err)
		}
		saved[name] = img
	}
	return saved, nil
}

// SaveConcurrent saves images in a concurrent, bounded manner.
func SaveConcurrent(ctx context.Context, cl clayout.Path, m map[string]v1.Image) (map[string]v1.Image, error) {
	saved := map[string]v1.Image{}

	for name, img := range m {
		name, img := name, img
		desc, err := partial.Descriptor(img)
		if err != nil {
			return saved, err
		}
		annotations := map[string]string{
			ocispec.AnnotationBaseImageName: name,
		}
		desc.Annotations = annotations
		if err := cl.AppendDescriptor(*desc); err != nil {
			return saved, err
		}
	}

	eg, _ := errgroup.WithContext(ctx)
	eg.SetLimit(10)

	for name, img := range m {
		name, img := name, img
		eg.Go(func() error {
			if err := cl.WriteImage(img); err != nil {
				return err
			}
			saved[name] = img
			return nil
		})
	}

	return saved, eg.Wait()
}
