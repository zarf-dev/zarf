package ocizarf

import (
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *ZarfOrasRemote) FetchZarfYAML() (pkg types.ZarfPackage, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return pkg, err
	}
	return oci.FetchYAMLFile[types.ZarfPackage](o.FetchLayer, manifest, layout.ZarfYAML)
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *ZarfOrasRemote) FetchImagesIndex() (index *ocispec.Index, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return index, err
	}
	return oci.FetchJSONFile[*ocispec.Index](o.FetchLayer, manifest, ZarfPackageIndexPath)
}
