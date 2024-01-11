package filters

import "github.com/defenseunicorns/zarf/src/types"

type ComponentFilterStrategy interface {
	Apply([]types.ZarfComponent) ([]types.ZarfComponent, error)
}
