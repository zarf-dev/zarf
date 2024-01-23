package ocizarf

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ZarfOrasRemote struct {
	*oci.OrasRemote
}

type Modifier func(*oci.OrasRemote)

func NewZarfOrasRemote(url string, mod ...oci.Modifier) (*ZarfOrasRemote, error) {
	remote, err := oci.NewOrasRemote(url, message.Infof, mod...)
	if err != nil {
		return nil, err
	}
	return &ZarfOrasRemote{remote}, nil
}

// WithSkeletonArch sets the target architecture for the remote to skeleton
func WithSkeletonArch() oci.Modifier {
	return oci.WithTargetPlatform(&ocispec.Platform{
		OS:           oci.MultiOS,
		Architecture: oci.SkeletonArch,
	})
}
