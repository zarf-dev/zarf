package images

import (
	"strings"

	"github.com/containerd/containerd/reference/docker"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
)

func PushAll(imageTarballPath string, buildImageList []string) {
	logrus.Info("Loading images")
	cranePlatformOptions := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})

	for _, src := range buildImageList {
		logContext := logrus.WithField("image", src)
		logContext.Info("Updating image")
		img, err := crane.LoadTag(imageTarballPath, src, cranePlatformOptions)
		if err != nil {
			logContext.Error(err)
			logContext.Warn("Unable to load the image from the update package")
			return
		}

		onlineName, err := docker.ParseNormalizedNamed(src)
		if err != nil {
			logContext.Warn("Unable to parse the image domain")
			return
		}

		offlineName := strings.Replace(src, docker.Domain(onlineName), config.ZarfLocalIP, 1)
		logrus.Info(offlineName)
		err = crane.Push(img, offlineName, cranePlatformOptions)
		if err != nil {
			logContext.Error(err)
			logContext.Warn("Unable to push the image to the registry")
		}
	}
}
