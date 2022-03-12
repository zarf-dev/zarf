package images

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/google/go-containerregistry/pkg/crane"
)

func Copy(src string, dest string) {
	if err := crane.Copy(src, dest, config.GetCraneOptions()); err != nil {
		message.Fatal(err, "Unable to copy the image")
	}
}
