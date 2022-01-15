package images

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
)

func PushAll(imageTarballPath string, buildImageList []string) {

	// Esabalish a registry tunnel to send the images
	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect(k8s.ZarfRegistry, false)

	for _, src := range buildImageList {
		message.Infof("Updating image %s -> %s", src, config.ZarfRegistry)
		img, err := crane.LoadTag(imageTarballPath, src, cranePlatformOptions)
		if err != nil {
			message.Error(err, "Unable to load the image from the update package")
			return
		}

		offlineName := utils.SwapHost(src, config.ZarfRegistry)

		err = crane.Push(img, offlineName, cranePlatformOptions)
		if err != nil {
			message.Error(err, "Unable to push the image to the registry")
		}
	}

	tunnel.Close()
}
