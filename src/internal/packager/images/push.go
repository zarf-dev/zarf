// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images
package images

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/google/go-containerregistry/pkg/crane"
)

// PushToZarfRegistry pushes a provided image into the configured Zarf registry
// This function will optionally shorten the image name while appending a checksum of the original image name
func (i *ImgConfig) PushToZarfRegistry() error {
	message.Debugf("images.PushToZarfRegistry(%#v)", i)

	registryURL := ""
	if i.RegInfo.InternalRegistry {
		// Establish a registry tunnel to send the images to the zarf registry
		tunnel := cluster.NewZarfTunnel()
		tunnel.Connect(cluster.ZarfRegistry, false)
		defer tunnel.Close()

		registryURL = tunnel.Endpoint()
	} else {
		registryURL = i.RegInfo.Address

		// If this is a serviceURL, create a port-forward tunnel to that resource
		if tunnel, err := cluster.NewTunnelFromServiceURL(registryURL); err != nil {
			message.Debug(err)
		} else {
			tunnel.Connect("", false)
			defer tunnel.Close()
			registryURL = tunnel.Endpoint()
		}
	}

	spinner := message.NewProgressSpinner("Storing images in the zarf registry")
	defer spinner.Stop()

	pushOptions := config.GetCraneAuthOption(i.RegInfo.PushUsername, i.RegInfo.PushPassword)
	message.Debugf("crane pushOptions = %#v", pushOptions)

	for _, src := range i.ImgList {
		spinner.Updatef("Updating image %s", src)
		img, err := crane.LoadTag(i.TarballPath, src, config.GetCraneOptions(i.Insecure)...)
		if err != nil {
			return err
		}
		offlineName := ""
		if i.NoChecksum {
			offlineName, err = utils.SwapHostWithoutChecksum(src, registryURL)
		} else {
			offlineName, err = utils.SwapHost(src, registryURL)
		}
		if err != nil {
			return err
		}

		message.Debugf("crane.Push() %s:%s -> %s)", i.TarballPath, src, offlineName)

		// TODO: @JPERRY if the `i.RegInfo` is an empty struct this is not returning an error.. That's pretty suspicious..
		if err = crane.Push(img, offlineName, pushOptions); err != nil {
			return err
		}
	}

	spinner.Success()
	return nil
}
