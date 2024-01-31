package ocizarf

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// SkeletonArch is the architecture used for skeleton packages
	SkeletonArch = "skeleton"
)

type ZarfOrasRemote struct {
	*oci.OrasRemote
}

type Modifier func(*oci.OrasRemote)

func NewZarfOrasRemote(url string, platform ocispec.Platform, mod ...oci.Modifier) (*ZarfOrasRemote, error) {
	modifiers := append(mod, oci.WithMediaType(ZarfConfigMediaType))
	remote, err := oci.NewOrasRemote(url, message.Infof, platform, modifiers...)
	if err != nil {
		return nil, err
	}
	return &ZarfOrasRemote{remote}, nil
}

// PlatformForSkeleton returns a skeleton oci
func PlatformForSkeleton() ocispec.Platform {
	return ocispec.Platform{
		OS:           oci.MultiOS,
		Architecture: SkeletonArch,
	}
}
