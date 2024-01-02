// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

type migrateRequiredToOptional struct {
	component types.ZarfComponent
}

func (m migrateRequiredToOptional) name() string {
	return RequiredToOptional
}

// If the component has already been migrated, clear the deprecated required.
func (m migrateRequiredToOptional) postbuild() types.ZarfComponent {
	c := m.component
	c.DeprecatedRequired = nil
	return c
}

// migrate converts the deprecated required to the new optional
func (m migrateRequiredToOptional) migrate() (types.ZarfComponent, string) {
	c := m.component
	if c.DeprecatedRequired == nil {
		return c, ""
	}

	switch *c.DeprecatedRequired {
	case true:
		c.Optional = nil
	case false:
		c.Optional = helpers.BoolPtr(true)
	}

	return c, ""
}
