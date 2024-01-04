// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/types"
)

// RequiredToOptionalID is the ID of the RequiredToOptional migration
const RequiredToOptionalID = "required-to-optional"

// RequiredToOptional migrates required to optional
type RequiredToOptional struct{}

// ID returns the ID of the migration
func (m RequiredToOptional) ID() string {
	return RequiredToOptionalID
}

// Clear the deprecated required.
func (m RequiredToOptional) Clear(mc types.ZarfComponent) types.ZarfComponent {
	mc.DeprecatedRequired = nil
	if mc.Optional != nil && *mc.Optional {
		mc.Optional = nil
	}
	return mc
}

// Run is a no-op for this migration.
func (m RequiredToOptional) Run(c types.ZarfComponent) (types.ZarfComponent, string) {
	if c.DeprecatedRequired == nil {
		return c, ""
	}

	warning := fmt.Sprintf("Component %q is using \"required\" which will be removed in Zarf v1.0.0. Please migrate to \"optional\".", c.Name)

	return c, warning
}
