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

	spinner := message.NewProgressSpinner("Storing images in the zarf registry")
	defer spinner.Stop()

	for _, src := range buildImageList {
		spinner.Updatef("Updating image %s", src)
		img, err := crane.LoadTag(imageTarballPath, src, config.GetCraneOptions())
		if err != nil {
			spinner.Errorf(err, "Unable to load the image from the update package")
			return
		}

		offlineName := utils.SwapHost(src, target)

		err = crane.Push(img, offlineName, config.GetCraneOptions())
		if err != nil {
			spinner.Errorf(err, "Unable to push the image to the registry")
		}
	}

	spinner.Success()
}
