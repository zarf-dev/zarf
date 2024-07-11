// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/types"
)

// migrateScriptsToActions coverts the deprecated scripts to the new actions
// The following have no migration:
// - Actions.Create.After
// - Actions.Remove.*
// - Actions.*.OnSuccess
// - Actions.*.OnFailure
// - Actions.*.*.Env
func migrateScriptsToActions(c types.ZarfComponent) (types.ZarfComponent, string) {
	var hasScripts bool

	if len(c.DeprecatedScripts.Prepare) > 0 {
		hasScripts = true
		for _, s := range c.DeprecatedScripts.Prepare {
			c.Actions = append(c.Actions, types.ZarfComponentAction{
				Cmd:  s,
				When: types.BeforeCreate,
			})
		}
	}

	if len(c.DeprecatedScripts.Before) > 0 {
		hasScripts = true
		for _, s := range c.DeprecatedScripts.Before {
			c.Actions = append(c.Actions, types.ZarfComponentAction{
				Cmd:  s,
				When: types.BeforeDeploy,
			})
		}
	}

	if len(c.DeprecatedScripts.After) > 0 {
		hasScripts = true
		for _, s := range c.DeprecatedScripts.After {
			c.Actions = append(c.Actions, types.ZarfComponentAction{
				Cmd:  s,
				When: types.AfterDeploy,
			})
		}
	}

	// Leave deprecated scripts in place, but warn users
	if hasScripts {
		return c, fmt.Sprintf("Component %q is using scripts which will be removed in Zarf v1.0.0. Please migrate to actions.", c.Name)
	}

	return c, ""
}
