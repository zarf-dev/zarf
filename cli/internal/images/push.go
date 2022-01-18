package images

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
)

func PushToZarfRegistry(imageTarballPath string, buildImageList []string, target string) {

	// Establish a registry tunnel to send the images if pushing to the zarf registry
	if target == config.ZarfRegistry {
		tunnel := k8s.NewZarfTunnel()
		tunnel.Connect(k8s.ZarfRegistry, false)
		defer tunnel.Close()
	}

	for _, src := range buildImageList {
		message.Infof("Updating image %s -> %s", src, target)
		img, err := crane.LoadTag(imageTarballPath, src, cranePlatformAMD64, cranePlatformARM64)
		if err != nil {
			message.Error(err, "Unable to load the image from the update package")
			return
		}

		offlineName := utils.SwapHost(src, target)

		err = crane.Push(img, offlineName, cranePlatformAMD64, cranePlatformARM64)
		if err != nil {
			message.Error(err, "Unable to push the image to the registry")
		}
	}
}
