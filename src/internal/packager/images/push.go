// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
)

// PushToZarfRegistry pushes a provided image into the configured Zarf registry
// This function will optionally shorten the image name while appending a checksum of the original image name.
func (i *ImgConfig) PushToZarfRegistry() error {
	message.Debugf("images.PushToZarfRegistry(%#v)", i)

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
		tunnel.Connect(target, false)
		defer tunnel.Close()
		registryURL = tunnel.Endpoint()
	} else {
		registryURL = i.RegInfo.Address
	}

	spinner := message.NewProgressSpinner("Storing images in the zarf registry")
	defer spinner.Stop()

	pushOptions := config.GetCraneOptions(i.Insecure)
	pushOptions = append(pushOptions, config.GetCraneAuthOption(i.RegInfo.PushUsername, i.RegInfo.PushPassword))

	message.Debugf("crane pushOptions = %#v", pushOptions)

	for _, src := range i.ImgList {
		spinner.Updatef("Updating image %s", src)

		img, err := i.LoadImageFromPackage(src)
		if err != nil {
			return err
		}

		// If this is not a no checksum image push it for use with the Zarf agent
		if !i.NoChecksum {
			offlineNameCRC, err := utils.SwapHost(src, registryURL)
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
		offlineName, err := utils.SwapHostWithoutChecksum(src, registryURL)
		if err != nil {
			return err
		}

		message.Debugf("crane.Push() %s:%s -> %s)", i.ImagesPath, src, offlineName)

		if err = crane.Push(img, offlineName, pushOptions...); err != nil {
			return err
		}
	}

	spinner.Success()
	return nil
}
