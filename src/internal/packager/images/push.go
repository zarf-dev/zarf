// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// Push pushes images to a registry.
func Push(ctx context.Context, cfg PushConfig) error {
	l := logger.From(ctx)
	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	toPush := map[transform.Image]v1.Image{}
	// Build an image list from the references
	for _, refInfo := range cfg.ImageList {
		img, err := utils.LoadOCIImage(cfg.SourceDirectory, refInfo)
		if err != nil {
			return err
		}
		toPush[refInfo] = img
	}

	var (
		err         error
		tunnel      *cluster.Tunnel
		registryURL = cfg.RegInfo.Address
	)
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
		pushOptions := createPushOpts(cfg)

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
			message.Infof("Pushing %s", refInfo.Reference)
			l.Info("pushing image", "name", refInfo.Reference)
			// If this is not a no checksum image push it for use with the Zarf agent
			if !cfg.NoChecksum {
				offlineNameCRC, err := transform.ImageTransformHost(registryURL, refInfo.Reference)
				if err != nil {
					return err
				}

				if err = pushImage(img, offlineNameCRC); err != nil {
					return err
				}
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
		}
		return nil
	}, retry.Context(ctx), retry.Attempts(uint(cfg.Retries)), retry.Delay(500*time.Millisecond))
	if err != nil {
		return err
	}

	return nil
}
