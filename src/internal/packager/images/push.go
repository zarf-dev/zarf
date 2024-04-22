// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// PushToZarfRegistry pushes a provided image into the configured Zarf registry
// This function will optionally shorten the image name while appending a checksum of the original image name.
func (i *ImageConfig) PushToZarfRegistry(ctx context.Context) error {
	message.Debug("images.PushToZarfRegistry()")

	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	refInfoToImage := map[transform.Image]v1.Image{}
	var totalSize int64
	// Build an image list from the references
	for _, refInfo := range i.ImageList {
		img, err := utils.LoadOCIImage(i.ImagesPath, refInfo)
		if err != nil {
			return err
		}
		refInfoToImage[refInfo] = img
		imgSize, err := calcImgSize(img)
		if err != nil {
			return err
		}
		totalSize += imgSize
	}

	// If this is not a no checksum image push we will be pushing two images (the second will go faster as it checks the same layers)
	if !i.NoChecksum {
		totalSize = totalSize * 2
	}

	httpTransport := http.DefaultTransport.(*http.Transport).Clone()
	httpTransport.TLSClientConfig.InsecureSkipVerify = i.Insecure
	// TODO (@WSTARR) This is set to match the TLSHandshakeTimeout to potentially mitigate effects of https://github.com/defenseunicorns/zarf/issues/1444
	httpTransport.ResponseHeaderTimeout = 10 * time.Second
	progressBar := message.NewProgressBar(totalSize, fmt.Sprintf("Pushing %d images to the zarf registry", len(i.ImageList)))
	defer progressBar.Stop()
	craneTransport := helpers.NewTransport(httpTransport, progressBar)

	pushOptions := config.GetCraneOptions(i.Insecure, i.Architectures...)
	pushOptions = append(pushOptions, config.GetCraneAuthOption(i.RegInfo.PushUsername, i.RegInfo.PushPassword))
	pushOptions = append(pushOptions, crane.WithTransport(craneTransport))

	var (
		err         error
		tunnel      *k8s.Tunnel
		registryURL string
	)

	registryURL = i.RegInfo.Address

	c, _ := cluster.NewCluster()
	if c != nil {
		registryURL, tunnel, err = c.ConnectToZarfRegistryEndpoint(ctx, i.RegInfo)
		if err != nil {
			return err
		}
	}

	if tunnel != nil {
		defer tunnel.Close()
	}

	pushImage := func(img v1.Image, name string) error {
		if tunnel != nil {
			return tunnel.Wrap(func() error { return crane.Push(img, name, pushOptions...) })
		}

		return crane.Push(img, name, pushOptions...)
	}

	for refInfo, img := range refInfoToImage {
		refTruncated := helpers.Truncate(refInfo.Reference, 55, true)
		progressBar.UpdateTitle(fmt.Sprintf("Pushing %s", refTruncated))

		// If this is not a no checksum image push it for use with the Zarf agent
		if !i.NoChecksum {
			offlineNameCRC, err := transform.ImageTransformHost(registryURL, refInfo.Reference)
			if err != nil {
				return err
			}

			message.Debugf("crane.Push() %s:%s -> %s)", i.ImagesPath, refInfo.Reference, offlineNameCRC)

			err = pushImage(img, offlineNameCRC)
			if err != nil {
				return err
			}
		}

		// To allow for other non-zarf workloads to easily see the images upload a non-checksum version
		// (this may result in collisions but this is acceptable for this use case)
		offlineName, err := transform.ImageTransformHostWithoutChecksum(registryURL, refInfo.Reference)
		if err != nil {
			return err
		}

		message.Debugf("crane.Push() %s:%s -> %s)", i.ImagesPath, refInfo.Reference, offlineName)

		err = pushImage(img, offlineName)
		if err != nil {
			return err
		}
	}

	progressBar.Successf("Pushed %d images to the zarf registry", len(i.ImageList))

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
