package images

import (
	"strings"

	"github.com/containerd/containerd/reference/docker"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sirupsen/logrus"
)

func PushAll(imageTarballPath string, buildImageList []string, targetHost string) {
	logrus.Info("Loading images")
	cranePlatformOptions := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})

	for _, src := range buildImageList {
		logContext := logrus.WithFields(logrus.Fields{
			"source": src,
			"target": targetHost,
		})
		logContext.Info("Updating image")
		img, err := crane.LoadTag(imageTarballPath, src, cranePlatformOptions)
		if err != nil {
			logContext.Debug(err)
			logContext.Warn("Unable to load the image from the update package")
			return
		}

		offlineName := SwapHost(src, targetHost)

		logrus.Info(offlineName)
		err = crane.Push(img, offlineName, cranePlatformOptions)
		if err != nil {
			logContext.Debug(err)
			logContext.Warn("Unable to push the image to the registry")
		}
	}
}

func SwapHost(src string, targetHost string) string {
	onlineName, err := docker.ParseNormalizedNamed(src)
	if err != nil {
		logContext := logrus.WithField("image", src)
		logContext.Debug(err)
		logContext.Warn("Unable to parse the image domain")
		return ""
	}

	result := strings.Replace(src, docker.Domain(onlineName), targetHost, 1)
	return result
}
