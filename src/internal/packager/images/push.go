// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"fmt"
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// PushToZarfRegistry pushes a provided image into the configured Zarf registry
// This function will optionally shorten the image name while appending a checksum of the original image name.
func (i *ImgConfig) PushToZarfRegistry() error {
	message.Debug("images.PushToZarfRegistry()")

	logs.Warn.SetOutput(&message.DebugWriter{})
	logs.Progress.SetOutput(&message.DebugWriter{})

	imageMap := map[string]v1.Image{}
	var totalSize int64
	// Build an image list from the references
	for _, src := range i.ImgList {
		img, err := i.LoadImageFromPackage(src)
		if err != nil {
			return err
		}
		imageMap[src] = img
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
	progressBar := message.NewProgressBar(totalSize, fmt.Sprintf("Pushing %d images to the zarf registry", len(i.ImgList)))
	craneTransport := utils.NewTransport(httpTransport, progressBar)

	pushOptions := config.GetCraneOptions(i.Insecure, i.Architectures...)
	pushOptions = append(pushOptions, config.GetCraneAuthOption(i.RegInfo.PushUsername, i.RegInfo.PushPassword))
	pushOptions = append(pushOptions, crane.WithTransport(craneTransport))
	message.Debugf("crane pushOptions = %#v", pushOptions)

	var (
		err         error
		tunnel      *cluster.Tunnel
		registryURL string
		target      string
	)

	if i.RegInfo.InternalRegistry {
		// Establish a registry tunnel to send the images to the zarf registry
		if tunnel, err = cluster.NewZarfTunnel(); err != nil {
			return err
		}
		target = cluster.ZarfRegistry
	} else {
		svcInfo, err := cluster.ServiceInfoFromNodePortURL(i.RegInfo.Address)

		// If this is a service (no error getting svcInfo), create a port-forward tunnel to that resource
		if err == nil {
			if tunnel, err = cluster.NewTunnel(svcInfo.Namespace, cluster.SvcResource, svcInfo.Name, 0, svcInfo.Port); err != nil {
				return err
			}
		}
	}

	if tunnel != nil {
		err = tunnel.Connect(target, false)
		if err != nil {
			return err
		}
		registryURL = tunnel.Endpoint()
		defer tunnel.Close()
	} else {
		registryURL = i.RegInfo.Address
	}

	for src, img := range imageMap {
		progressBar.UpdateTitle(fmt.Sprintf("Pushing %s", src))

		// If this is not a no checksum image push it for use with the Zarf agent
		if !i.NoChecksum {
			offlineNameCRC, err := transform.ImageTransformHost(registryURL, src)
			if err != nil {
				return err
			}

			message.Debugf("crane.Push() %s:%s -> %s)", i.ImagesPath, src, offlineNameCRC)

			if err = crane.Push(img, offlineNameCRC, pushOptions...); err != nil {
				return err
			}
		}

		// To allow for other non-zarf workloads to easily see the images upload a non-checksum version
		// (this may result in collisions but this is acceptable for this use case)
		offlineName, err := transform.ImageTransformHostWithoutChecksum(registryURL, src)
		if err != nil {
			return err
		}

		message.Debugf("crane.Push() %s:%s -> %s)", i.ImagesPath, src, offlineName)

		if err = crane.Push(img, offlineName, pushOptions...); err != nil {
			return err
		}
	}

	progressBar.Successf("Pushed %d images to the zarf registry", len(i.ImgList))

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
