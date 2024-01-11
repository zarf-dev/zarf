package creator

import (
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// verify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)
)

type PackageCreator struct{}

func (p *PackageCreator) CdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error {
	return cdToBaseDir(createOpts, cwd)
}
