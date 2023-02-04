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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

var ErrNoDockerClient = errors.New("no docker client available")

// PullAll pulls all of the images in the provided tag map.
func (i *ImgConfig) PullAll() error {
	var (
		longer   string
		imgCount = len(i.ImgList)
		imageMap = map[string]v1.Image{}
	)

	// If docker is permitted, try to pull images with docker first.
	if !i.NoDockerPull {
		// Try to load the docker client.
		cli, err := client.NewClientWithOpts(client.FromEnv)

		// If Docker client is available, try to pull images with docker.
		if err == nil && cli.ClientVersion() != "" {
			// If the pull fails, continue with crane.
			if err := i.pullImagesWithDocker(cli); err != nil {
				message.Debugf("Failed to pull images with docker: %s", err)
			} else {
				// Otherwise, return nil as the pull was successful.
				return nil
			}
		}

		// Otherwise, continue with crane.
	}

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

		img, err := i.PullImage(src)
		if err != nil {
			return fmt.Errorf("failed to pull image %s: %w", src, err)
		}
		imageMap[src] = img
	}

	spinner.Updatef("Creating image tarball (this will take a while)")

	tagToImage := map[name.Tag]v1.Image{}

	for src, img := range imageMap {
		tag, err := name.NewTag(src, name.WeakValidation)
		if err != nil {
			return fmt.Errorf("failed to create tag for image %s: %w", src, err)
		}
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

// PullImage returns a v1.Image either by loading a local tarball or the wider internet
func (i *ImgConfig) PullImage(src string) (v1.Image, error) {
	// Load image tarballs from the local filesystem
	if strings.HasSuffix(src, ".tar") || strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		message.Debugf("loading image tarball: %s", src)
		return crane.Load(src, config.GetCraneOptions(true)...)
	}

	// We were unable to pull from the local daemon, so attempt to pull from the wider internet
	img, err := crane.Pull(src, config.GetCraneOptions(i.Insecure)...)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", src, err)
	}

	message.Debugf("loading image with cache: %s", src)
	imageCachePath := filepath.Join(config.GetAbsCachePath(), config.ZarfImageCacheDir)
	img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))

	return img, nil
}

// TODO: (@jeff-mccoy) enable --inescure flag support for pullImagesWithDocker (will work with local images, but not remote).
func (i *ImgConfig) pullImagesWithDocker(cli *client.Client) error {
	spinner := message.NewProgressSpinner("Pulling %d images via Docker.", len(i.ImgList))
	defer spinner.Stop()

	platform := fmt.Sprintf("--platform=linux/%s", config.GetArch())

	// Try to pull all images with docker.
	for _, img := range i.ImgList {
		spinner.SetWriterPrefixf("Docker pull %s:  ", img)
		execCfg := exec.Config{
			Stdout: spinner,
			Stderr: spinner,
		}
		_, _, err := exec.CmdWithContext(context.TODO(), execCfg, "docker", "pull", img, platform)
		if err != nil {
			message.Debugf("image pull with Docker failed: %s", err.Error())
			continue
		}
	}

	var totalSize int64

	// Get the total size of all images.
	for _, img := range i.ImgList {
		spinner.Updatef("Reading image size: %s", img)
		imgInfo, _, err := cli.ImageInspectWithRaw(context.Background(), img)
		if err != nil {
			message.Debugf("image inspect with Docker failed: %s", err.Error())
		}
		message.Debug(message.JSONValue(imgInfo))
		totalSize += imgInfo.Size
	}
	spinner.Success()

	prettySize := utils.ByteFormat(float64(totalSize), 2)
	progressBar := message.NewProgressBar(totalSize, "Storing %d images via Docker (%s)", len(i.ImgList), prettySize)
	defer progressBar.Stop()

	respBody, err := cli.ImageSave(context.Background(), i.ImgList)
	if err != nil {
		return fmt.Errorf("image save with Docker failed: %w", err)
	}
	defer respBody.Close()

	// Create a new tarball
	tarball, err := os.Create(i.TarballPath)
	if err != nil {
		return fmt.Errorf("unable to create tarball: %w", err)
	}
	defer tarball.Close()

	// Copy the tarball from the docker daemon to disk
	if _, err = io.Copy(tarball, io.TeeReader(respBody, progressBar)); err != nil {
		return fmt.Errorf("unable to save tarball: %w", err)
	}

	return nil
}
