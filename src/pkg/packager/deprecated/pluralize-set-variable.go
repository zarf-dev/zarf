// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func migrateSetVariableToSetVariables(c v1alpha1.ZarfComponent) (v1alpha1.ZarfComponent, string) {
	hasSetVariable := false

	migrate := func(actions []v1alpha1.ZarfComponentAction) []v1alpha1.ZarfComponentAction {
		for i := range actions {
			if actions[i].DeprecatedSetVariable != "" && len(actions[i].SetVariables) < 1 {
				hasSetVariable = true
				actions[i].SetVariables = []v1alpha1.Variable{
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

	// Leave deprecated setVariable in place, but warn users
	if hasSetVariable {
		return c, fmt.Sprintf("Component '%s' is using setVariable in actions which will be removed in Zarf v1.0.0. Please migrate to the list form of setVariables.", c.Name)
	}

	return c, ""
}

func clearSetVariables(c v1alpha1.ZarfComponent) v1alpha1.ZarfComponent {
	clear := func(actions []v1alpha1.ZarfComponentAction) []v1alpha1.ZarfComponentAction {
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
