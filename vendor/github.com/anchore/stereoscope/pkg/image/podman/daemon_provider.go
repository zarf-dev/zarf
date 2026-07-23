package podman

import (
	"github.com/moby/moby/client"

	"github.com/anchore/stereoscope/internal/podman"
	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
	"github.com/anchore/stereoscope/pkg/image/docker"
)

const Daemon image.Source = image.PodmanDaemonSource

func NewDaemonProvider(tmpDirGen *file.TempDirGenerator, imageStr string, platform *image.Platform) image.Provider {
	return docker.NewAPIClientProvider(Daemon, tmpDirGen, imageStr, platform, func() (client.APIClient, error) {
		return podman.GetClient()
	})
}
