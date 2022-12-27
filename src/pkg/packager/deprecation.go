// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import "github.com/defenseunicorns/zarf/src/types"

// migrateScriptsToActions coverts the deprecated scripts to the new actions
// The following have no migration:
// - Actions.Create.Last
// - Actions.Remove.*
// - Actions.*.Success
// - Actions.*.Failure
// - Actions.*.*.Env
func migrateScriptsToActions(c types.ZarfComponent) types.ZarfComponent {
	// Function to convert a script string to a ZarfComponentAction
	genScript := func(s string) types.ZarfComponentAction {
		return types.ZarfComponentAction{
			// ShowOutput (default false) -> Mute (default false)
			Mute: !c.DeprecatedScripts.ShowOutput,
			// TimeoutSeconds -> MaxSeconds
			MaxSeconds: c.DeprecatedScripts.TimeoutSeconds,
			// Retry unchanged
			Retry: c.DeprecatedScripts.Retry,
			// Script entry -> Cmd
			Cmd: s,
		}
	}

	// Scripts.Prepare -> Actions.Create.First
	for _, s := range c.DeprecatedScripts.Prepare {
		c.Actions.Create.First = append(c.Actions.Create.First, genScript(s))
	}

	// Scripts.Before -> Actions.Deploy.First
	for _, s := range c.DeprecatedScripts.Before {
		c.Actions.Deploy.First = append(c.Actions.Deploy.First, genScript(s))
	}

	// Scripts.After -> Actions.Deploy.Last
	for _, s := range c.DeprecatedScripts.After {
		c.Actions.Deploy.Last = append(c.Actions.Deploy.Last, genScript(s))
	}

	// Clear the deprecated scripts
	c.DeprecatedScripts = types.DeprecatedZarfComponentScripts{}

	return c
}
