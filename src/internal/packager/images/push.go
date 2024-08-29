// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// Push pushes images to a registry.
func Push(ctx context.Context, cfg PushConfig) error {
	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	toPush := map[transform.Image]v1.Image{}
	var totalSize int64
	// Build an image list from the references
	for _, refInfo := range cfg.ImageList {
		img, err := utils.LoadOCIImage(cfg.SourceDirectory, refInfo)
		if err != nil {
			return err
		}
		toPush[refInfo] = img
		imgSize, err := calcImgSize(img)
		if err != nil {
			return err
		}
		totalSize += imgSize
	}

	// If this is not a no checksum image push we will be pushing two images (the second will go faster as it checks the same layers)
	if !cfg.NoChecksum {
		totalSize = totalSize * 2
	}

	var (
		err         error
		tunnel      *cluster.Tunnel
		registryURL = cfg.RegInfo.Address
	)

	progress := message.NewProgressBar(totalSize, fmt.Sprintf("Pushing %d images", len(toPush)))
	defer progress.Close()

	err = retry.Do(func() error {
		c, _ := cluster.NewCluster()
		if c != nil {
			registryURL, tunnel, err = c.ConnectToZarfRegistryEndpoint(ctx, cfg.RegInfo)
			if err != nil {
				return err
			}
			if tunnel != nil {
				defer tunnel.Close()
			}
		}

		progress = message.NewProgressBar(totalSize, fmt.Sprintf("Pushing %d images", len(toPush)))
		pushOptions := createPushOpts(cfg, progress)

		pushImage := func(img v1.Image, name string) error {
			if tunnel != nil {
				return tunnel.Wrap(func() error { return crane.Push(img, name, pushOptions...) })
			}

			return crane.Push(img, name, pushOptions...)
		}

		pushed := []transform.Image{}
		defer func() {
			for _, refInfo := range pushed {
				delete(toPush, refInfo)
			}
		}()
		for refInfo, img := range toPush {
			refTruncated := helpers.Truncate(refInfo.Reference, 55, true)
			progress.Updatef(fmt.Sprintf("Pushing %s", refTruncated))

			size, err := calcImgSize(img)
			if err != nil {
				return err
			}

			// If this is not a no checksum image push it for use with the Zarf agent
			if !cfg.NoChecksum {
				offlineNameCRC, err := transform.ImageTransformHost(registryURL, refInfo.Reference)
				if err != nil {
					return err
				}

				if err = pushImage(img, offlineNameCRC); err != nil {
					return err
				}

				totalSize -= size
			}

			// To allow for other non-zarf workloads to easily see the images upload a non-checksum version
			// (this may result in collisions but this is acceptable for this use case)
			offlineName, err := transform.ImageTransformHostWithoutChecksum(registryURL, refInfo.Reference)
			if err != nil {
				return err
			}

			if err = pushImage(img, offlineName); err != nil {
				return err
			}

			pushed = append(pushed, refInfo)
			totalSize -= size
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(uint(cfg.Retries)), retry.Delay(500*time.Millisecond))
	if err != nil {
		return err
	}

	progress.Successf("Pushed %d images", len(cfg.ImageList))

	return nil
}

func calcImgSize(img v1.Image) (int64, error) {
	size, err := img.Size()
	if err != nil {
		return size, err
	}

	layers, err := img.Layers()
	if err != nil {
		return size, err
	}

	for _, layer := range layers {
		ls, err := layer.Size()
		if err != nil {
			return size, err
		}
		size += ls
	}

	return size, nil
}
