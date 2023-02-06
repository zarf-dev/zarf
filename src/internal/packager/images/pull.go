// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// PullAll pulls all of the images in the provided tag map.
func (i *ImgConfig) PullAll() error {
	var (
		longer     string
		imgCount   = len(i.ImgList)
		imageMap   = map[string]v1.Image{}
		tagToImage = map[name.Tag]v1.Image{}
		totalSize  int64
	)

	// Give some additional user feedback on larger image sets
	if imgCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imgCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	spinner := message.NewProgressSpinner("Loading metadata for %d images. %s", imgCount, longer)
	defer spinner.Stop()

	if message.GetLogLevel() >= message.DebugLevel {
		logs.Warn.SetOutput(spinner)
		logs.Progress.SetOutput(spinner)
	}

	for idx, src := range i.ImgList {
		spinner.Updatef("Fetching image metadata (%d of %d): %s", idx+1, imgCount, src)

		img, err := i.PullImage(src, spinner)
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", src, err)
		}
		imageMap[src] = img
	}

	for src, img := range imageMap {
		tag, err := name.NewTag(src, name.WeakValidation)
		if err != nil {
			return fmt.Errorf("failed to create tag for image %s: %w", src, err)
		}
		size, _ := img.Size()
		totalSize += size
		tagToImage[tag] = img
	}
	spinner.Success()

	var (
		progress    = make(chan v1.Update, 200)
		progressBar *message.ProgressBar
		title       string
	)

	go func() {
		_ = tarball.MultiWriteToFile(i.TarballPath, tagToImage, tarball.WithProgress(progress))
	}()

	for update := range progress {
		switch {
		case update.Error != nil && errors.Is(update.Error, io.EOF):
			progressBar.Success("Pulling %d images (%s)", len(imageMap), utils.ByteFormat(float64(update.Total), 2))
			return nil
		case update.Error != nil && strings.HasPrefix(update.Error.Error(), "archive/tar: missed writing "):
			// Handle potential image cache corruption with a more helpful error. See L#54 in libexec/src/archive/tar/writer.go
			message.Warnf("Potential image cache corruption: %s of %v bytes - try clearing cache with \"zarf tools clear-cache\"", update.Error.Error(), update.Total)
			return fmt.Errorf("failed to write image tarball: %w", update.Error)
		case update.Error != nil:
			return fmt.Errorf("failed to write image tarball: %w", update.Error)
		default:
			title = fmt.Sprintf("Pulling %d images (%s of %s)", len(imageMap),
				utils.ByteFormat(float64(update.Complete), 2),
				utils.ByteFormat(float64(update.Total), 2),
			)
			if progressBar == nil {
				progressBar = message.NewProgressBar(update.Total, title)
			}
			progressBar.Update(update.Complete, title)
		}
	}

	return nil
}

// PullImage returns a v1.Image either by loading a local tarball or the wider internet.
func (i *ImgConfig) PullImage(src string, spinner *message.Spinner) (img v1.Image, err error) {
	// Load image tarballs from the local filesystem.
	if strings.HasSuffix(src, ".tar") || strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		spinner.Updatef("Reading image tarball: %s", src)
		return crane.Load(src, config.GetCraneOptions(true)...)
	}

	// If crane is unable to pull the image, try to load it from the local docker daemon.
	if _, err := crane.Manifest(src, config.GetCraneOptions(i.Insecure)...); err != nil {
		message.Debugf("crane unable to pull image %s: %s", src, err)
		spinner.Updatef("%s not found, trying with docker instead. This may take some time.", src)

		reference, err := name.ParseReference(src)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", src, err)
		}

		// Use unbuffered opener to avoid OOM Kill issues https://github.com/defenseunicorns/zarf/issues/1214.
		// This will also take for ever to load large images.
		if img, err = daemon.Image(reference, daemon.WithUnbufferedOpener()); err != nil {
			return nil, fmt.Errorf("failed to load image %s from docker daemon: %w", src, err)
		}

		// If we were able to pull from the local daemon, return the image.
		return img, err
	}

	// We were unable to pull from the local daemon, so attempt to pull from the wider internet
	if img, err = crane.Pull(src, config.GetCraneOptions(i.Insecure)...); err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", src, err)
	}

	spinner.Updatef("Preparing imagce %s", src)
	imageCachePath := filepath.Join(config.GetAbsCachePath(), config.ZarfImageCacheDir)
	img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))

	return img, nil
}
