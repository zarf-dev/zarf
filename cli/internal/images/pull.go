package images

import (
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/sirupsen/logrus"
)

const cachePath = ".image-cache"

func PullAll(buildImageList []string, imageTarballPath string) {
	logrus.Info("Loading images")
	cranePlatformOptions := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})
	imageMap := map[string]v1.Image{}

	for _, src := range buildImageList {
		logContext := logrus.WithField("image", src)
		logContext.Info("Fetching image metadata")
		img, err := crane.Pull(src, cranePlatformOptions)
		if err != nil {
			logContext.Warn("Unable to pull the image")
		}
		img = cache.Image(img, cache.NewFilesystemCache(cachePath))
		imageMap[src] = img
	}

	logrus.Info("Creating image tarball (this will take a while)")
	if err := crane.MultiSave(imageMap, imageTarballPath); err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to save the tarball")
	}
}
