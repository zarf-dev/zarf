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

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
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
	clayout "github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	ctypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/moby/moby/client"
)

// ImgInfo wraps references/information about an image
type ImgInfo struct {
	RefInfo        transform.Image
	Img            v1.Image
	HasImageLayers bool
}

// PullAll pulls all of the images in the provided tag map.
func (i *ImageConfig) PullAll() ([]ImgInfo, error) {
	var (
		longer            string
		imageCount        = len(i.ImageList)
		refInfoToImage    = map[transform.Image]v1.Image{}
		referenceToDigest = make(map[string]string)
		imgInfoList       []ImgInfo
	)

	type digestInfo struct {
		refInfo transform.Image
		digest  string
	}

	// Give some additional user feedback on larger image sets
	if imageCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imageCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	spinner := message.NewProgressSpinner("Loading metadata for %d images. %s", imageCount, longer)
	defer spinner.Stop()

	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	metadataImageConcurrency := helpers.NewConcurrencyTools[ImgInfo, error](len(i.ImageList))

	defer metadataImageConcurrency.Cancel()

	spinner.Updatef("Fetching image metadata (0 of %d)", len(i.ImageList))

	// Spawn a goroutine for each image to load its metadata
	for _, refInfo := range i.ImageList {
		// Create a closure so that we can pass the src into the goroutine
		refInfo := refInfo
		go func() {

			if metadataImageConcurrency.IsDone() {
				return
			}

			actualSrc := refInfo.Reference
			for k, v := range i.RegistryOverrides {
				if strings.HasPrefix(refInfo.Reference, k) {
					actualSrc = strings.Replace(refInfo.Reference, k, v, 1)
				}
			}

			if metadataImageConcurrency.IsDone() {
				return
			}

			img, hasImageLayers, err := i.PullImage(actualSrc, spinner)
			if err != nil {
				metadataImageConcurrency.ErrorChan <- fmt.Errorf("failed to pull %s: %w", actualSrc, err)
				return
			}

			if metadataImageConcurrency.IsDone() {
				return
			}

			metadataImageConcurrency.ProgressChan <- ImgInfo{RefInfo: refInfo, Img: img, HasImageLayers: hasImageLayers}
		}()
	}

	onMetadataProgress := func(finishedImage ImgInfo, iteration int) {
		spinner.Updatef("Fetching image metadata (%d of %d): %s", iteration+1, len(i.ImageList), finishedImage.RefInfo.Reference)
		refInfoToImage[finishedImage.RefInfo] = finishedImage.Img
		imgInfoList = append(imgInfoList, finishedImage)
	}

	onMetadataError := func(err error) error {
		return err
	}

	if err := metadataImageConcurrency.WaitWithProgress(onMetadataProgress, onMetadataError); err != nil {
		return nil, err
	}

	// Create the ImagePath directory
	if err := helpers.CreateDirectory(i.ImagesPath, helpers.ReadExecuteAllWriteUser); err != nil {
		return nil, fmt.Errorf("failed to create image path %s: %w", i.ImagesPath, err)
	}

	totalBytes := int64(0)
	processedLayers := make(map[string]v1.Layer)
	for refInfo, img := range refInfoToImage {
		// Get the byte size for this image
		layers, err := img.Layers()
		if err != nil {
			return nil, fmt.Errorf("unable to get layers for image %s: %w", refInfo.Reference, err)
		}
		for _, layer := range layers {
			layerDigest, err := layer.Digest()
			if err != nil {
				return nil, fmt.Errorf("unable to get digest for image layer: %w", err)
			}

			// Only calculate this layer size if we haven't already looked at it
			if _, ok := processedLayers[layerDigest.Hex]; !ok {
				size, err := layer.Size()
				if err != nil {
					return nil, fmt.Errorf("unable to get size of layer: %w", err)
				}
				totalBytes += size
				processedLayers[layerDigest.Hex] = layer
			}

		}
	}
	spinner.Updatef("Preparing image sources and cache for image pulling")

	// Create special sauce crane Path object
	// If it already exists use it
	cranePath, err := clayout.FromPath(i.ImagesPath)
	// Use crane pattern for creating OCI layout if it doesn't exist
	if err != nil {
		// If it doesn't exist create it
		cranePath, err = clayout.Write(i.ImagesPath, empty.Index)
		if err != nil {
			return nil, err
		}
	}

	for refInfo, img := range refInfoToImage {
		imgDigest, err := img.Digest()
		if err != nil {
			return nil, fmt.Errorf("unable to get digest for image %s: %w", refInfo.Reference, err)
		}
		referenceToDigest[refInfo.Reference] = imgDigest.String()
	}

	spinner.Success()

	// Create a thread to update a progress bar as we save the image files to disk
	doneSaving := make(chan error)
	updateText := fmt.Sprintf("Pulling %d images", imageCount)
	go utils.RenderProgressBarForLocalDirWrite(i.ImagesPath, totalBytes, doneSaving, updateText, updateText)

	imageSavingConcurrency := helpers.NewConcurrencyTools[digestInfo, error](len(refInfoToImage))

	defer imageSavingConcurrency.Cancel()

	// Spawn a goroutine for each image to write it's config and manifest to disk using crane
	for refInfo, img := range refInfoToImage {
		// Create a closure so that we can pass the refInfo and img into the goroutine
		refInfo, img := refInfo, img
		go func() {
			if err := cranePath.WriteImage(img); err != nil {
				// Check if the cache has been invalidated, and warn the user if so
				if strings.HasPrefix(err.Error(), "error writing layer: expected blob size") {
					message.Warnf("Potential image cache corruption: %s - try clearing cache with \"zarf tools clear-cache\"", err.Error())
				}
				imageSavingConcurrency.ErrorChan <- fmt.Errorf("error when trying to save the img (%s): %w", refInfo.Reference, err)
				return
			}

			if imageSavingConcurrency.IsDone() {
				return
			}

			// Get the image digest so we can set an annotation in the image.json later
			imgDigest, err := img.Digest()
			if err != nil {
				imageSavingConcurrency.ErrorChan <- err
				return
			}

			if imageSavingConcurrency.IsDone() {
				return
			}

			imageSavingConcurrency.ProgressChan <- digestInfo{digest: imgDigest.String(), refInfo: refInfo}
		}()
	}

	onImageSavingProgress := func(finishedImage digestInfo, _ int) {
		referenceToDigest[finishedImage.refInfo.Reference] = finishedImage.digest
	}

	onImageSavingError := func(err error) error {
		// Send a signal to the progress bar that we're done and wait for the thread to finish
		doneSaving <- err
		<-doneSaving
		message.WarnErr(err, "Failed to write image config or manifest, trying again up to 3 times...")
		return err
	}

	if err := imageSavingConcurrency.WaitWithProgress(onImageSavingProgress, onImageSavingError); err != nil {
		return nil, err
	}

	// for every image sequentially append OCI descriptor
	for refInfo, img := range refInfoToImage {
		desc, err := partial.Descriptor(img)
		if err != nil {
			return nil, err
		}

		if err := cranePath.AppendDescriptor(*desc); err != nil {
			return nil, err
		}

		imgDigest, err := img.Digest()
		if err != nil {
			return nil, err
		}

		referenceToDigest[refInfo.Reference] = imgDigest.String()
	}

	if err := utils.AddImageNameAnnotation(i.ImagesPath, referenceToDigest); err != nil {
		return nil, fmt.Errorf("unable to format OCI layout: %w", err)
	}

	// Send a signal to the progress bar that we're done and wait for the thread to finish
	doneSaving <- nil
	<-doneSaving

	return imgInfoList, nil
}

// PullImage returns a v1.Image either by loading a local tarball or pulling from the wider internet.
func (i *ImageConfig) PullImage(src string, spinner *message.Spinner) (img v1.Image, hasImageLayers bool, err error) {
	cacheImage := false
	// Load image tarballs from the local filesystem.
	if strings.HasSuffix(src, ".tar") || strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		spinner.Updatef("Reading image tarball: %s", src)
		img, err = crane.Load(src, config.GetCraneOptions(true, i.Architectures...)...)
		if err != nil {
			return nil, false, err
		}
	} else if desc, err := crane.Get(src, config.GetCraneOptions(i.Insecure)...); err != nil {
		// If crane is unable to pull the image, try to load it from the local docker daemon.
		message.Notef("Falling back to local 'docker' images, failed to find the manifest on a remote: %s", err.Error())

		// Parse the image reference to get the image name.
		reference, err := name.ParseReference(src)
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse image reference: %w", err)
		}

		// Attempt to connect to the local docker daemon.
		ctx := context.TODO()
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, false, fmt.Errorf("docker not available: %w", err)
		}
		cli.NegotiateAPIVersion(ctx)

		// Inspect the image to get the size.
		rawImg, _, err := cli.ImageInspectWithRaw(ctx, src)
		if err != nil {
			return nil, false, fmt.Errorf("failed to inspect image via docker: %w", err)
		}

		// Warn the user if the image is large.
		if rawImg.Size > 750*1000*1000 {
			message.Warnf("%s is %s and may take a very long time to load via docker. "+
				"See https://docs.zarf.dev/docs/faq for suggestions on how to improve large local image loading operations.",
				src, utils.ByteFormat(float64(rawImg.Size), 2))
		}

		// Use unbuffered opener to avoid OOM Kill issues https://github.com/defenseunicorns/zarf/issues/1214.
		// This will also take for ever to load large images.
		if img, err = daemon.Image(reference, daemon.WithUnbufferedOpener()); err != nil {
			return nil, false, fmt.Errorf("failed to load image from docker daemon: %w", err)
		}
	} else {
		refInfo, err := transform.ParseImageRef(src)
		if err != nil {
			return nil, false, err
		}

		if refInfo.Digest != "" && (desc.MediaType == ctypes.OCIImageIndex || desc.MediaType == ctypes.DockerManifestList) {
			var idx v1.IndexManifest
			if err := json.Unmarshal(desc.Manifest, &idx); err != nil {
				return nil, false, fmt.Errorf("%w: %w", lang.ErrUnsupportedImageType, err)
			}
			imageOptions := "please select one of the images below based on your platform to use instead"
			imageBaseName := refInfo.Name
			if refInfo.Tag != "" {
				imageBaseName = fmt.Sprintf("%s:%s", imageBaseName, refInfo.Tag)
			}
			for _, manifest := range idx.Manifests {
				imageOptions = fmt.Sprintf("%s\n %s@%s for platform %s", imageOptions, imageBaseName, manifest.Digest, manifest.Platform)
			}
			return nil, false, fmt.Errorf("%w: %s", lang.ErrUnsupportedImageType, imageOptions)
		}

		// Manifest was found, so use crane to pull the image.
		if img, err = crane.Pull(src, config.GetCraneOptions(i.Insecure, i.Architectures...)...); err != nil {
			return nil, false, fmt.Errorf("failed to pull image: %w", err)
		}
		cacheImage = true
	}

	hasImageLayers, err = utils.HasImageLayers(img)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check image layer mediatype: %w", err)
	}

	if hasImageLayers && cacheImage {
		spinner.Updatef("Preparing image %s", src)
		imageCachePath := filepath.Join(config.GetAbsCachePath(), layout.ImagesDir)
		img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))
	}

	return img, hasImageLayers, nil

}
