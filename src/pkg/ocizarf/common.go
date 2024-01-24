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

func NewZarfOrasRemote(url string, platform ocispec.Platform, mod ...oci.Modifier) (*ZarfOrasRemote, error) {
	remote, err := oci.NewOrasRemote(url, message.Infof, platform, mod...)
	if err != nil {
		return nil, err
	}
	return &ZarfOrasRemote{remote}, nil
}
