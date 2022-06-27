package images

import (
	"errors"
	"fmt"
	"io"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func PullAll(buildImageList []string, imageTarballPath string) map[name.Tag]v1.Image {
	var (
		longer     string
		imageCount = len(buildImageList)
	)

	// Give some additional user feedback on larger image sets
	if imageCount > 15 {
		longer = "This step may take a couple of minutes to complete."
	} else if imageCount > 5 {
		longer = "This step may take several seconds to complete."
	}

	spinner := message.NewProgressSpinner("Loading metadata for %d images. %s", imageCount, longer)
	defer spinner.Stop()

	imageMap := map[string]v1.Image{}

	if message.GetLogLevel() >= message.DebugLevel {
		logs.Warn.SetOutput(spinner)
		logs.Progress.SetOutput(spinner)
	}

	for idx, src := range buildImageList {
		spinner.Updatef("Fetching image metadata (%d of %d): %s", idx+1, imageCount, src)
		img, err := crane.Pull(src, config.GetCraneOptions()...)
		if err != nil {
			spinner.Fatalf(err, "Unable to pull the image %s", src)
		}
		imageCachePath := config.GetImageCachePath()
		img = cache.Image(img, cache.NewFilesystemCache(imageCachePath))
		imageMap[src] = img
	}

	spinner.Updatef("Creating image tarball (this will take a while)")

	tagToImage := map[name.Tag]v1.Image{}

	for src, img := range imageMap {
		ref, err := name.ParseReference(src)
		if err != nil {
			spinner.Fatalf(err, "parsing ref %q", src)
		}

		tag, ok := ref.(name.Tag)
		if !ok {
			d, ok := ref.(name.Digest)
			if !ok {
				spinner.Fatalf(nil, "image reference %s wasn't a tag or digest", src)
			}
			tag = d.Repository.Tag("digest-only")
		}
		tagToImage[tag] = img
	}
	spinner.Success()

	progress := make(chan v1.Update, 200)

	go func() {
		_ = tarball.MultiWriteToFile(imageTarballPath, tagToImage, tarball.WithProgress(progress))
	}()

	var progressBar *message.ProgressBar
	var title string

	for update := range progress {
		switch {
		case update.Error != nil && errors.Is(update.Error, io.EOF):
			progressBar.Success("Pulling %v images (%s)", len(imageMap), utils.ByteFormat(float64(update.Total), 2))
			return tagToImage
		case update.Error != nil:
			message.Fatal(update.Error, "error writing image tarball")
		default:
			title = fmt.Sprintf("Pulling %v images (%s of %s)", len(imageMap),
				utils.ByteFormat(float64(update.Complete), 2),
				utils.ByteFormat(float64(update.Total), 2),
			)
			if progressBar == nil {
				progressBar = message.NewProgressBar(update.Total, title)
			}
			progressBar.Update(update.Complete, title)
		}
	}

	return tagToImage
}
