// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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

		img, err := pullImage(src, i.Insecure)
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

// pullImage returns a v1.Image either by loading a local tarball, the pulling from the local daemon, or the wider internet
func pullImage(src string, insecure bool) (v1.Image, error) {
	var img v1.Image
	var err error

	// Load image tarballs from the local filesystem
	if strings.HasSuffix(src, ".tar") || strings.HasSuffix(src, ".tar.gz") || strings.HasSuffix(src, ".tgz") {
		img, err = crane.Load(src, config.GetCraneOptions(true)...)
		return img, err
	}

	// Attempt to pull the image from the local daemon
	reference, err := name.ParseReference(src)
	if err != nil {
		// log this error but don't return the error since we can still try pulling from the wider internet
		message.Debugf("unable to parse the image reference, this might have impacts on pulling from the local daemon: %s", err.Error())
	}
	img, err = daemon.Image(reference, daemon.WithContext(context.Background()))
	if err != nil {
		return img, err
	}

	// We were unable to pull from the local daemon, so attempt to pull from the wider internet
	img, err = crane.Pull(src, config.GetCraneOptions(insecure)...)
	return img, err
}

// FormatCraneOCILayout ensures that all images are in the OCI format.
func FormatCraneOCILayout(ociPath string) error {
	type IndexJSON struct {
		SchemaVersion int `json:"schemaVersion"`
		Manifests     []struct {
			MediaType string `json:"mediaType"`
			Size      int    `json:"size"`
			Digest    string `json:"digest"`
		} `json:"manifests"`
	}

	indexJSON, err := os.Open(path.Join(ociPath, "index.json"))
	if err != nil {
		message.Errorf(err, "Unable to open %s/index.json", ociPath)
		return err
	}
	var index IndexJSON
	byteValue, _ := io.ReadAll(indexJSON)
	json.Unmarshal(byteValue, &index)

	digest := strings.TrimPrefix(index.Manifests[0].Digest, "sha256:")
	b, err := os.ReadFile(path.Join(ociPath, "blobs", "sha256", digest))
	if err != nil {
		message.Errorf(err, "Unable to open %s/blobs/sha256/%s", ociPath, digest)
		return err
	}
	manifest := string(b)
	// replace all docker media types w/ oci media types
	manifest = strings.ReplaceAll(manifest, "application/vnd.docker.distribution.manifest.v2+json", "application/vnd.oci.image.manifest.v1+json")
	manifest = strings.ReplaceAll(manifest, "application/vnd.docker.image.rootfs.diff.tar.gzip", "application/vnd.oci.image.layer.v1.tar+gzip")

	h := sha256.New()
	h.Write([]byte(manifest))
	bs := h.Sum(nil)

	// Write the manifest to the blobs directory w/ the sha256 hash as the filename
	manifestPath := path.Join(ociPath, "blobs", "sha256", fmt.Sprintf("%x", bs))
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		message.Errorf(err, "Unable to create %s/blobs/sha256/%x", ociPath, bs)
		return err
	}
	defer manifestFile.Close()
	_, err = manifestFile.WriteString(manifest)
	if err != nil {
		message.Errorf(err, "Unable to write to %s/blobs/sha256/%x", ociPath, bs)
		return err
	}

	// Update the index.json to point to the new manifest
	index.SchemaVersion = 2
	index.Manifests[0].Digest = fmt.Sprintf("sha256:%x", bs)
	index.Manifests[0].Size = len(manifest)
	index.Manifests[0].MediaType = "application/vnd.oci.image.manifest.v1+json"
	indexJSON.Close()
	_ = os.Remove(path.Join(ociPath, "index.json"))
	indexJSON, err = os.Create(path.Join(ociPath, "index.json"))
	if err != nil {
		message.Errorf(err, "Unable to create %s/index.json", ociPath)
		return err
	}
	indexJSONBytes, err := json.Marshal(index)
	if err != nil {
		message.Errorf(err, "Unable to marshal %s/index.json", ociPath)
		return err
	}
	_, err = indexJSON.Write(indexJSONBytes)
	if err != nil {
		message.Errorf(err, "Unable to write to %s/index.json", ociPath)
		return err
	}
	indexJSON.Close()

	return nil
}
