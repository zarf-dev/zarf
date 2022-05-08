package images

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
)

func PushToZarfRegistry(imageTarballPath string, buildImageList []string) {
	message.Debugf("images.PushToZarfRegistry(%v, %v)", imageTarballPath, buildImageList)

	// Establish a registry tunnel to send the images if pushing to the zarf registry
	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect(k8s.ZarfRegistry, false)
	defer tunnel.Close()

	spinner := message.NewProgressSpinner("Storing images in the zarf registry")
	defer spinner.Stop()

	pushOptions := config.GetCraneAuthOption(config.ZarfRegistryPushUser, config.GetSecret(config.StateRegistryPush))
	message.Debug(pushOptions)

	for _, src := range buildImageList {
		spinner.Updatef("Updating image %s", src)
		img, err := crane.LoadTag(imageTarballPath, src, config.GetCraneOptions())
		if err != nil {
			spinner.Errorf(err, "Unable to load the image from the update package")
			return
		}

		offlineName := utils.SwapHost(src, config.ZarfRegistry)
		err = crane.Push(img, offlineName, pushOptions)

		if err != nil {
			spinner.Fatalf(err, "Unable to push the image to the registry")
		}
	}

	spinner.Success()
}
