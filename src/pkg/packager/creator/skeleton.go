package creator

import (
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// verify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

type SkeletonCreator struct{}

func (p *SkeletonCreator) CdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error {
	return cdToBaseDir(createOpts, cwd)
}
