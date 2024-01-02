// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

type migrateRequiredToOptional struct{}

func (m migrateRequiredToOptional) name() string {
	return RequiredToOptional
}

// If the component has already been migrated, clear the deprecated required.
func (m migrateRequiredToOptional) clear(mc types.ZarfComponent) types.ZarfComponent {
	mc.DeprecatedRequired = nil
	return mc
}

// run converts the deprecated required to the new optional
func (m migrateRequiredToOptional) run(c types.ZarfComponent) (types.ZarfComponent, string) {
	if c.DeprecatedRequired == nil {
		return c, ""
	}

	switch *c.DeprecatedRequired {
	case true:
		c.Optional = nil
	case false:
		c.Optional = helpers.BoolPtr(true)
	}

	return c, fmt.Sprintf("Component %q is using \"required\" which will be removed in Zarf v1.0.0. Please migrate to \"optional\". Please migrate to \"optional\".", c.Name)
}
