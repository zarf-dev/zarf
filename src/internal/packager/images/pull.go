// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images
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
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func (i *ImgConfig) PullAll() (map[name.Tag]v1.Image, error) {
	var (
		longer   string
		imgCount = len(i.ImgList)
	)

	// Give some additional user feedback on larger image sets
	if imgCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imgCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	spinner := message.NewProgressSpinner("Loading metadata for %d images. %s", imgCount, longer)
	defer spinner.Stop()

	imageMap := map[string]v1.Image{}

	if message.GetLogLevel() >= message.DebugLevel {
		logs.Warn.SetOutput(spinner)
		logs.Progress.SetOutput(spinner)
	}

	for idx, src := range i.ImgList {
		spinner.Updatef("Fetching image metadata (%d of %d): %s", idx+1, imgCount, src)
		img, err := crane.Pull(src, config.GetCraneOptions(i.Insecure)...)
		if err != nil {
			return nil, fmt.Errorf("failed to pull image %s: %w", src, err)
		}
		imageCachePath := filepath.Join(config.GetAbsCachePath(), config.ZarfImageCacheDir)
		img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))
		imageMap[src] = img
	}

	spinner.Updatef("Creating image tarball (this will take a while)")

	tagToImage := map[name.Tag]v1.Image{}

	for src, img := range imageMap {
		ref, err := name.ParseReference(src)
		if err != nil {
			return nil, fmt.Errorf("failed to parse image reference %s: %w", src, err)
		}

		tag, ok := ref.(name.Tag)
		if !ok {
			d, ok := ref.(name.Digest)
			if !ok {
				return nil, fmt.Errorf("image reference %s wasn't a tag or digest", src)
			}
			tag = d.Repository.Tag("digest-only")
		}
		tagToImage[tag] = img
	}
	spinner.Success()

	progress := make(chan v1.Update, 200)

	go func() {
		_ = tarball.MultiWriteToFile(i.TarballPath, tagToImage, tarball.WithProgress(progress))
	}()

	var progressBar *message.ProgressBar
	var title string

	for update := range progress {
		switch {
		case update.Error != nil && errors.Is(update.Error, io.EOF):
			progressBar.Success("Pulling %d images (%s)", len(imageMap), utils.ByteFormat(float64(update.Total), 2))
			return tagToImage, nil
		case update.Error != nil && strings.HasPrefix(update.Error.Error(), "archive/tar: missed writing "):
			// Handle potential image cache corruption with a more helpful error. See L#54 in libexec/src/archive/tar/writer.go
			message.Warnf("Potential image cache corruption: %s of %v bytes - try clearing cache with \"zarf tools clear-cache\"", update.Error.Error(), update.Total)
			return nil, fmt.Errorf("failed to write image tarball: %w", update.Error)
		case update.Error != nil:
			return nil, fmt.Errorf("failed to write image tarball: %w", update.Error)
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

	return tagToImage, nil
}
