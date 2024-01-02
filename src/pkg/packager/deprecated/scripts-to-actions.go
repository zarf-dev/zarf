// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"
	"math"

	"github.com/defenseunicorns/zarf/src/types"
)

type migrateScriptsToActions struct {
	component types.ZarfComponent
}

func (m migrateScriptsToActions) name() string {
	return ScriptsToActions
}

// If the component has already been migrated, clear the deprecated scripts.
func (m migrateScriptsToActions) postbuild() types.ZarfComponent {
	mc := m.component
	mc.DeprecatedScripts = types.DeprecatedZarfComponentScripts{}
	return mc
}

// migrate coverts the deprecated scripts to the new actions
// The following have no migration:
// - Actions.Create.After
// - Actions.Remove.*
// - Actions.*.OnSuccess
// - Actions.*.OnFailure
// - Actions.*.*.Env
func (m migrateScriptsToActions) migrate() (types.ZarfComponent, string) {
	mc := m.component
	var hasScripts bool

	// Convert a script configs to action defaults.
	defaults := types.ZarfComponentActionDefaults{
		// ShowOutput (default false) -> Mute (default false)
		Mute: !mc.DeprecatedScripts.ShowOutput,
		// TimeoutSeconds -> MaxSeconds
		MaxTotalSeconds: mc.DeprecatedScripts.TimeoutSeconds,
	}

	// Retry is now an integer vs a boolean (implicit infinite retries), so set to an absurdly high number
	if mc.DeprecatedScripts.Retry {
		defaults.MaxRetries = math.MaxInt
	}

	// Scripts.Prepare -> Actions.Create.Before
	if len(mc.DeprecatedScripts.Prepare) > 0 {
		hasScripts = true
		mc.Actions.OnCreate.Defaults = defaults
		for _, s := range mc.DeprecatedScripts.Prepare {
			mc.Actions.OnCreate.Before = append(mc.Actions.OnCreate.Before, types.ZarfComponentAction{Cmd: s})
		}
	}

	// Scripts.Before -> Actions.Deploy.Before
	if len(mc.DeprecatedScripts.Before) > 0 {
		hasScripts = true
		mc.Actions.OnDeploy.Defaults = defaults
		for _, s := range mc.DeprecatedScripts.Before {
			mc.Actions.OnDeploy.Before = append(mc.Actions.OnDeploy.Before, types.ZarfComponentAction{Cmd: s})
		}
	}

	// Scripts.After -> Actions.Deploy.After
	if len(mc.DeprecatedScripts.After) > 0 {
		hasScripts = true
		mc.Actions.OnDeploy.Defaults = defaults
		for _, s := range mc.DeprecatedScripts.After {
			mc.Actions.OnDeploy.After = append(mc.Actions.OnDeploy.After, types.ZarfComponentAction{Cmd: s})
		}
	}

	// Leave deprecated scripts in place, but warn users
	if hasScripts {
		return mc, fmt.Sprintf("Component '%s' is using scripts which will be removed in Zarf v1.0.0. Please migrate to actions.", mc.Name)
	}

	return mc, ""
}
