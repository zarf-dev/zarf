package images

import (
	"github.com/defenseunicorns/zarf/cli/internal/log"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
)

const cachePath = ".image-cache"

func PullAll(buildImageList []string, imageTarballPath string) {
	log.Logger.Info("Loading images")
	cranePlatformOptions := crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})
	imageMap := map[string]v1.Image{}

	for _, src := range buildImageList {
		logContext := log.Logger.WithField("image", src)
		logContext.Info("Fetching image metadata")
		img, err := crane.Pull(src, cranePlatformOptions)
		if err != nil {
			logContext.Warn("Unable to pull the image")
		}
		img = cache.Image(img, cache.NewFilesystemCache(cachePath))
		imageMap[src] = img
	}

	log.Logger.Info("Creating image tarball (this will take a while)")
	if err := crane.MultiSave(imageMap, imageTarballPath); err != nil {
		log.Logger.Debug(err)
		log.Logger.Fatal("Unable to save the tarball")
	}
}
