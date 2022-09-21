package images

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
)

// PushToZarfRegistry pushes a provided image into the configured Zarf registry
// This function will optionally shorten the image name while appending a sha1sum of the original image name
func PushToZarfRegistry(imageTarballPath string, buildImageList []string, addShasumToImg bool) error {
	message.Debugf("images.PushToZarfRegistry(%s, %s)", imageTarballPath, buildImageList)

	registryUrl := ""
	if config.GetContainerRegistryInfo().InternalRegistry {
		// Establish a registry tunnel to send the images to the zarf registry
		tunnel := k8s.NewZarfTunnel()
		tunnel.Connect(k8s.ZarfRegistry, false)
		defer tunnel.Close()

		registryUrl = tunnel.Endpoint()
	} else {
		registryUrl = config.GetContainerRegistryInfo().Address

		// If this is a serviceURL, create a port-forward tunnel to that resource
		if k8s.IsServiceURL(registryUrl) {
			tunnel, err := k8s.NewTunnelFromServiceURL(registryUrl)
			if err != nil {
				return err
			}

			tunnel.Connect("", false)
			defer tunnel.Close()
			registryUrl = tunnel.Endpoint()
		}
	}

	spinner := message.NewProgressSpinner("Storing images in the zarf registry")
	defer spinner.Stop()

	pushOptions := config.GetCraneAuthOption(config.GetContainerRegistryInfo().PushUsername, config.GetContainerRegistryInfo().PushPassword)
	message.Debugf("crane pushOptions = %#v", pushOptions)

	for _, src := range buildImageList {
		spinner.Updatef("Updating image %s", src)
		img, err := crane.LoadTag(imageTarballPath, src, config.GetCraneOptions()...)
		if err != nil {
			return err
		}
		offlineName := ""
		if addShasumToImg {
			offlineName = utils.SwapHost(src, registryUrl)
		} else {
			offlineName = utils.SwapHostWithoutSha(src, registryUrl)
		}
		if err = crane.Push(img, offlineName, pushOptions); err != nil {
			return err
		}
	}

	spinner.Success()
	return nil
}
