// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
)

func migrateSetVariableToSetVariables(c types.ZarfComponent) types.ZarfComponent {
	migrate := func(actions []types.ZarfComponentAction) []types.ZarfComponentAction {
		for i := range actions {
			if actions[i].DeprecatedSetVariable != "" && len(actions[i].SetVariables) < 1 {
				actions[i].SetVariables = []variables.Variable{
					{
						Name:      actions[i].DeprecatedSetVariable,
						Sensitive: false,
					},
				}
			}
		}

		return actions
	}

	// Migrate OnCreate SetVariables
	c.Actions.OnCreate.After = migrate(c.Actions.OnCreate.After)
	c.Actions.OnCreate.Before = migrate(c.Actions.OnCreate.Before)
	c.Actions.OnCreate.OnSuccess = migrate(c.Actions.OnCreate.OnSuccess)
	c.Actions.OnCreate.OnFailure = migrate(c.Actions.OnCreate.OnFailure)

	// Migrate OnDeploy SetVariables
	c.Actions.OnDeploy.After = migrate(c.Actions.OnDeploy.After)
	c.Actions.OnDeploy.Before = migrate(c.Actions.OnDeploy.Before)
	c.Actions.OnDeploy.OnSuccess = migrate(c.Actions.OnDeploy.OnSuccess)
	c.Actions.OnDeploy.OnFailure = migrate(c.Actions.OnDeploy.OnFailure)

	// Migrate OnRemove SetVariables
	c.Actions.OnRemove.After = migrate(c.Actions.OnRemove.After)
	c.Actions.OnRemove.Before = migrate(c.Actions.OnRemove.Before)
	c.Actions.OnRemove.OnSuccess = migrate(c.Actions.OnRemove.OnSuccess)
	c.Actions.OnRemove.OnFailure = migrate(c.Actions.OnRemove.OnFailure)

	return c
}

func clearSetVariables(c types.ZarfComponent) types.ZarfComponent {
	clear := func(actions []types.ZarfComponentAction) []types.ZarfComponentAction {
		for i := range actions {
			actions[i].DeprecatedSetVariable = ""
		}

		return actions
	}

	// Clear OnCreate SetVariables
	c.Actions.OnCreate.After = clear(c.Actions.OnCreate.After)
	c.Actions.OnCreate.Before = clear(c.Actions.OnCreate.Before)
	c.Actions.OnCreate.OnSuccess = clear(c.Actions.OnCreate.OnSuccess)
	c.Actions.OnCreate.OnFailure = clear(c.Actions.OnCreate.OnFailure)

	// Clear OnDeploy SetVariables
	c.Actions.OnDeploy.After = clear(c.Actions.OnDeploy.After)
	c.Actions.OnDeploy.Before = clear(c.Actions.OnDeploy.Before)
	c.Actions.OnDeploy.OnSuccess = clear(c.Actions.OnDeploy.OnSuccess)
	c.Actions.OnDeploy.OnFailure = clear(c.Actions.OnDeploy.OnFailure)

	// Clear OnRemove SetVariables
	c.Actions.OnRemove.After = clear(c.Actions.OnRemove.After)
	c.Actions.OnRemove.Before = clear(c.Actions.OnRemove.Before)
	c.Actions.OnRemove.OnSuccess = clear(c.Actions.OnRemove.OnSuccess)
	c.Actions.OnRemove.OnFailure = clear(c.Actions.OnRemove.OnFailure)

	return c
}
