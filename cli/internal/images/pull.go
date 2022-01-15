package images

import (
	"errors"
	"fmt"
	"io"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
)

func PullAll(buildImageList []string, imageTarballPath string) {
	spinner := message.NewProgresSpinner("Loading metadata for %v images", len(buildImageList))
	defer spinner.Stop()

	imageMap := map[string]v1.Image{}

	if message.GetLogLevel() >= message.DebugLevel {
		logs.Warn.SetOutput(spinner)
		logs.Progress.SetOutput(spinner)
	}

	for _, src := range buildImageList {
		spinner.Updatef("Fetching image metadata for %v", src)
		img, err := crane.Pull(src, cranePlatformOptions)
		if err != nil {
			spinner.Fatalf(err, "Unable to pull the image %s", src)
		}
		img = cache.Image(img, cache.NewFilesystemCache(cachePath))
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

	var progressBar *pterm.ProgressbarPrinter
	var title string

	for update := range progress {
		switch {
		case update.Error != nil && errors.Is(update.Error, io.EOF):
			_, _ = progressBar.Stop()
			pterm.Success.Println(title)
			return
		case update.Error != nil:
			message.Fatal(update.Error, "error writing image tarball")
		default:
			if progressBar == nil {
				total := int(update.Total)
				title = fmt.Sprintf("Pulling %v images (%s)", len(imageMap), utils.ByteFormat(float64(total), 2))
				progressBar, _ = pterm.DefaultProgressbar.
					WithTotal(total).
					WithShowCount(false).
					WithTitle(title).
					WithRemoveWhenDone(true).
					Start()
			}
			chunk := int(update.Complete) - progressBar.Current
			progressBar.Add(chunk)
		}
	}
}
