package ocizarf

import (
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/types"
)

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *ZarfOrasRemote) FetchZarfYAML() (pkg types.ZarfPackage, err error) {
	manifest, err := o.FetchRoot()
	if err != nil {
		return pkg, err
	}
	return oci.FetchYAMLFile[types.ZarfPackage](o.FetchLayer, manifest, layout.ZarfYAML)
}
