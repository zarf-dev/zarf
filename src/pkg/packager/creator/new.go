package creator

import "github.com/defenseunicorns/zarf/src/types"

type Creator interface {
	CdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error
}

func New(createOpts *types.ZarfCreateOptions) Creator {
	if createOpts.IsSkeleton {
		return &SkeletonCreator{}
	}
	return &PackageCreator{}
}
