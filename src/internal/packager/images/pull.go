// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pterm/pterm"
)

// PullAll pulls all of the images in the provided tag map.
func (i *ImgConfig) PullAll() error {
	var (
		longer      string
		imgCount    = len(i.ImgList)
		imageMap    = map[string]v1.Image{}
		tagToImage  = map[name.Tag]v1.Image{}
		tagToDigest = make(map[string]string)
	)

	// Give some additional user feedback on larger image sets
	if imgCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imgCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	spinner := message.NewProgressSpinner("Loading metadata for %d images. %s", imgCount, longer)
	defer spinner.Stop()

	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	type srcAndImg struct {
		src string
		img v1.Image
	}

	metadataImageConcurrency := utils.NewConcurrencyTools[srcAndImg, error](len(i.ImgList))

	defer metadataImageConcurrency.Cancel()

	spinner.Updatef("Fetching image metadata (0 of %d)", len(i.ImgList))

	// Spawn a goroutine for each image to load its metadata
	for _, src := range i.ImgList {
		// Create a closure so that we can pass the src into the goroutine
		src := src
		go func() {
			// Make sure to call Done() on the WaitGroup when the goroutine finishes
			defer metadataImageConcurrency.WaitGroupDone()

			srcParsed, err := transform.ParseImageRef(src)
			if err != nil {
				metadataImageConcurrency.ErrorChan <- fmt.Errorf("failed to parse image ref %s: %w", src, err)
				return
			}

			actualSrc := src
			if overrideHost, present := i.RegistryOverrides[srcParsed.Host]; present {
				actualSrc, err = transform.ImageTransformHostWithoutChecksum(overrideHost, src)
				if err != nil {
					metadataImageConcurrency.ErrorChan <- fmt.Errorf("failed to swap override host %s for %s: %w", overrideHost, src, err)
					return
				}
			}

			img, err := i.PullImage(actualSrc, spinner)
			if err != nil {
				metadataImageConcurrency.ErrorChan <- fmt.Errorf("failed to pull image %s: %w", actualSrc, err)
				return
			}
			metadataImageConcurrency.ProgressChan <- srcAndImg{src: src, img: img}
		}()
	}

	metadataImageConcurrency.OnProgress = func(finishedImage srcAndImg, iteration int) {
		spinner.Updatef("Fetching image metadata (%d of %d): %s", iteration+1, len(i.ImgList), finishedImage.src)
		imageMap[finishedImage.src] = finishedImage.img
	}

	metadataImageConcurrency.OnError = func(err error) error {
		return err
	}

	if err := metadataImageConcurrency.Wait(); err != nil {
		return err
	}

	// Create the ImagePath directory
	if err := utils.CreateDirectory(i.ImagesPath, 0755); err != nil {
		return fmt.Errorf("failed to create image path %s: %w", i.ImagesPath, err)
	}

	totalBytes := int64(0)
	processedLayers := make(map[string]bool)
	for src, img := range imageMap {
		tag, err := name.NewTag(src, name.WeakValidation)
		if err != nil {
			return fmt.Errorf("failed to create tag for image %s: %w", src, err)
		}
		tagToImage[tag] = img
		// Get the byte size for this image
		layers, err := img.Layers()
		if err != nil {
			return fmt.Errorf("unable to get layers for image %s: %w", src, err)
		}
		for _, layer := range layers {
			layerDigest, err := layer.Digest()
			if err != nil {
				return fmt.Errorf("unable to get digest for image layer: %w", err)
			}

			// Only calculate this layer size if we haven't already looked at it
			if !processedLayers[layerDigest.Hex] {
				size, err := layer.Size()
				if err != nil {
					return fmt.Errorf("unable to get size of layer: %w", err)
				}
				totalBytes += size
				processedLayers[layerDigest.Hex] = true
			}

		}
	}
	spinner.Updatef("Preparing image sources and cache for image pulling")
	spinner.Success()

	// Create a thread to update a progress bar as we save the image files to disk
	doneSaving := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go utils.RenderProgressBarForLocalDirWrite(i.ImagesPath, totalBytes, &wg, doneSaving, fmt.Sprintf("Pulling %d images", imgCount))

	type digestAndTag struct {
		digest string
		tag    string
	}

	// Create special sauce crane Path object

	// If it already exists use it
	cranePath, err := layout.FromPath(i.ImagesPath)
	// Use crane pattern for creating OCI layout if it doesn't exist
	if err != nil {
		// If it doesn't exist create it
		cranePath, err = layout.Write(i.ImagesPath, empty.Index)
		if err != nil {
			return err
		}
	}

	imageSavingConcurrency := utils.NewConcurrencyTools[digestAndTag, error](len(tagToImage))

	defer imageSavingConcurrency.Cancel()

	// Spawn a goroutine for each image to write it's layers/manifests/etc to disk using crane
	for tag, img := range tagToImage {
		// Create a closure so that we can pass the tag and img into the goroutine
		tag, img := tag, img
		go func() {
			// Make sure to call Done() on the WaitGroup when the goroutine finishes
			defer imageSavingConcurrency.WaitGroupDone()
			// Save the image via crane
			err := cranePath.WriteImage(img)
			if err != nil {
				// Check if the cache has been invalidated, and warn the user if so
				if strings.HasPrefix(err.Error(), "error writing layer: expected blob size") {
					message.Warnf("Potential image cache corruption: %s - try clearing cache with \"zarf tools clear-cache\"", err.Error())
				}
				imageSavingConcurrency.ErrorChan <- fmt.Errorf("error when trying to save the img (%s): %w", tag.Name(), err)
				return
			}

			// Get the image digest so we can set an annotation in the image.json later
			imgDigest, err := img.Digest()
			if err != nil {
				imageSavingConcurrency.ErrorChan <- err
				return
			}
			imageSavingConcurrency.ProgressChan <- digestAndTag{digest: imgDigest.String(), tag: tag.String()}
		}()
	}

	imageSavingConcurrency.OnError = func(err error) error {
		return err
	}

	imageSavingConcurrency.OnProgress = func(finishedImage digestAndTag, iteration int) {
		tagToDigest[finishedImage.tag] = finishedImage.digest
	}

	if err := imageSavingConcurrency.Wait(); err != nil {
		return err
	}

	// for every image sequentially append OCI descriptor

	for tag, img := range tagToImage {
		desc, err := partial.Descriptor(img)
		if err != nil {
			return err
		}

		cranePath.AppendDescriptor(*desc)
		if err != nil {
			return err
		}

		imgDigest, err := img.Digest()
		if err != nil {
			return err
		}

		tagToDigest[tag.String()] = imgDigest.String()
	}

	if err := addImageNameAnnotation(i.ImagesPath, tagToDigest); err != nil {
		return fmt.Errorf("unable to format OCI layout: %w", err)
	}

	// Send a signal to the progress bar that we're done and ait for the thread to finish
	doneSaving <- 1
	wg.Wait()

	return err
}

// PullImage returns a v1.Image either by loading a local tarball or the wider internet.
func (i *ImgConfig) PullImage(src string, spinner *message.Spinner) (img v1.Image, err error) {
	// Load image tarballs from the local filesystem.
	if strings.HasSuffix(src, ".tar") || strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		spinner.Updatef("Reading image tarball: %s", src)
		return crane.Load(src, config.GetCraneOptions(true, i.Architectures...)...)
	}

	// If crane is unable to pull the image, try to load it from the local docker daemon.
	if _, err := crane.Manifest(src, config.GetCraneOptions(i.Insecure, i.Architectures...)...); err != nil {
		message.Debugf("crane unable to pull image %s: %s", src, err)
		spinner.Updatef("Falling back to docker for %s. This may take some time.", src)

		// Parse the image reference to get the image name.
		reference, err := name.ParseReference(src)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", src, err)
		}

		// Attempt to connect to the local docker daemon.
		ctx := context.TODO()
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("docker not available: %w", err)
		}
		cli.NegotiateAPIVersion(ctx)

		// Inspect the image to get the size.
		rawImg, _, err := cli.ImageInspectWithRaw(ctx, src)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect image %s via docker: %w", src, err)
		}

		// Warn the user if the image is large.
		if rawImg.Size > 750*1000*1000 {
			warn := pterm.DefaultParagraph.WithMaxWidth(message.TermWidth).Sprintf("%s is %s and may take a very long time to load via docker. "+
				"See https://docs.zarf.dev/docs/faq for suggestions on how to improve large local image loading operations.",
				src, utils.ByteFormat(float64(rawImg.Size), 2))
			spinner.Warnf(warn)
		}

		// Use unbuffered opener to avoid OOM Kill issues https://github.com/defenseunicorns/zarf/issues/1214.
		// This will also take for ever to load large images.
		if img, err = daemon.Image(reference, daemon.WithUnbufferedOpener()); err != nil {
			return nil, fmt.Errorf("failed to load image %s from docker daemon: %w", src, err)
		}

		// The pull from the docker daemon was successful, return the image.
		return img, err
	}

	// Manifest was found, so use crane to pull the image.
	if img, err = crane.Pull(src, config.GetCraneOptions(i.Insecure, i.Architectures...)...); err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", src, err)
	}

	spinner.Updatef("Preparing image %s", src)
	imageCachePath := filepath.Join(config.GetAbsCachePath(), config.ZarfImageCacheDir)
	img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))

	return img, nil
}

// IndexJSON represents the index.json file in an OCI layout.
type IndexJSON struct {
	SchemaVersion int `json:"schemaVersion"`
	Manifests     []struct {
		MediaType   string            `json:"mediaType"`
		Size        int               `json:"size"`
		Digest      string            `json:"digest"`
		Annotations map[string]string `json:"annotations"`
	} `json:"manifests"`
}

// addImageNameAnnotation adds an annotation to the index.json file so that the deploying code can figure out what the image tag <-> digest shasum will be.
func addImageNameAnnotation(ociPath string, tagToDigest map[string]string) error {
	indexPath := filepath.Join(ociPath, "index.json")

	// Read the file contents and turn it into a usable struct that we can manipulate
	var index IndexJSON
	byteValue, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("unable to read the contents of the file (%s) so we can add an annotation: %w", indexPath, err)
	}
	if err = json.Unmarshal(byteValue, &index); err != nil {
		return fmt.Errorf("unable to process the conents of the file (%s): %w", indexPath, err)
	}

	// Loop through the manifests and add the appropriate OCI Base Image Name Annotation
	for idx, manifest := range index.Manifests {
		if manifest.Annotations == nil {
			manifest.Annotations = make(map[string]string)
		}

		var baseImageName string

		for tag, digest := range tagToDigest {
			if digest == manifest.Digest {
				baseImageName = tag
			}
		}

		if baseImageName != "" {
			manifest.Annotations[ocispec.AnnotationBaseImageName] = baseImageName
			index.Manifests[idx] = manifest
			delete(tagToDigest, baseImageName)
		}
	}

	// Write the file back to the package
	indexJSONBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, indexJSONBytes, 0600)
}
