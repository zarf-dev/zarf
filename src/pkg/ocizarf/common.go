package ocizarf

import (
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// SkeletonArch is the architecture used for skeleton packages
	SkeletonArch = "skeleton"
	// MultiOS is the OS used for multi-platform packages
	MultiOS = "multi"
)

type ZarfOrasRemote struct {
	*oci.OrasRemote
}

// log is a function that logs a message.
// type log func(string, ...any)

// func NewZarfOrasRemote(url string, logger log, mod ...oci.Modifier) (*ZarfOrasRemote, error) {
// 	NewOrasRemote(url, logger, mod...)
// }

// WithSkeletonArch sets the target architecture for the remote to skeleton
func WithSkeletonArch() oci.Modifier {
	return oci.WithTargetPlatform(&ocispec.Platform{
		OS:           MultiOS,
		Architecture: SkeletonArch,
	})
}
