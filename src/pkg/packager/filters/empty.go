package filters

import "github.com/defenseunicorns/zarf/src/types"

var (
	_ ComponentFilterStrategy = &EmptyFilter{}
)

type EmptyFilter struct{}

func (f *EmptyFilter) Apply(components []types.ZarfComponent) ([]types.ZarfComponent, error) {
	return components, nil
}
