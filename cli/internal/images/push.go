package images

import (
	"regexp"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sirupsen/logrus"
)

func PushAll(imageTarballPath string, buildImageList []string, targetHost string) {
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

		err = crane.Push(img, offlineName, cranePlatformOptions)
		if err != nil {
			logContext.Debug(err)
			logContext.Warn("Unable to push the image to the registry")
		}
	}
}

// SwapHost Perform base url replacment without the docker libs
func SwapHost(src string, targetHost string) string {
	// For further explanation see https://regex101.com/library/PiL191 and https://regex101.com/r/PiL191/1
	var parser = regexp.MustCompile(`(?im)^([a-z0-9\-.]+\.[a-z0-9\-]+:?[0-9]*)?/?(.+)$`)
	var substitution = targetHost + "/$2"
	return parser.ReplaceAllString(src, substitution)
}
