// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"
	"math"

	"github.com/zarf-dev/zarf/src/types"
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

	// Convert a script configs to action defaults.
	defaults := types.ZarfComponentActionDefaults{
		// ShowOutput (default false) -> Mute (default false)
		Mute: !c.DeprecatedScripts.ShowOutput,
		// TimeoutSeconds -> MaxSeconds
		MaxTotalSeconds: c.DeprecatedScripts.TimeoutSeconds,
	}

	// Retry is now an integer vs a boolean (implicit infinite retries), so set to an absurdly high number
	if c.DeprecatedScripts.Retry {
		defaults.MaxRetries = math.MaxInt
	}

	// Scripts.Prepare -> Actions.Create.Before
	if len(c.DeprecatedScripts.Prepare) > 0 {
		hasScripts = true
		c.Actions.OnCreate.Defaults = defaults
		for _, s := range c.DeprecatedScripts.Prepare {
			c.Actions.OnCreate.Before = append(c.Actions.OnCreate.Before, types.ZarfComponentAction{Cmd: s})
		}
	}

	// Scripts.Before -> Actions.Deploy.Before
	if len(c.DeprecatedScripts.Before) > 0 {
		hasScripts = true
		c.Actions.OnDeploy.Defaults = defaults
		for _, s := range c.DeprecatedScripts.Before {
			c.Actions.OnDeploy.Before = append(c.Actions.OnDeploy.Before, types.ZarfComponentAction{Cmd: s})
		}
	}

	// Scripts.After -> Actions.Deploy.After
	if len(c.DeprecatedScripts.After) > 0 {
		hasScripts = true
		c.Actions.OnDeploy.Defaults = defaults
		for _, s := range c.DeprecatedScripts.After {
			c.Actions.OnDeploy.After = append(c.Actions.OnDeploy.After, types.ZarfComponentAction{Cmd: s})
		}
	}

	// Leave deprecated scripts in place, but warn users
	if hasScripts {
		return c, fmt.Sprintf("Component '%s' is using scripts which will be removed in Zarf v1.0.0. Please migrate to actions.", c.Name)
	}

	return c, ""
}
