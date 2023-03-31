// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// List of migrations tracked in the zarf.yaml build data.
const (
	ScriptsToActionsMigrated = "scripts-to-actions"
	PluralizeSetVariable     = "pluralize-set-variable"
)

// MigrateComponent runs all migrations on a component.
// Build should be empty on package create, but include just in case someone copied a zarf.yaml from a zarf package.
func MigrateComponent(build types.ZarfBuildData, c types.ZarfComponent) types.ZarfComponent {
	// If the component has already been migrated, clear the deprecated scripts.
	if utils.SliceContains(build.Migrations, ScriptsToActionsMigrated) {
		c.DeprecatedScripts = types.DeprecatedZarfComponentScripts{}
	} else {
		// Otherwise, run the migration.
		c = migrateScriptsToActions(c)
	}

	// If the component has already been migrated, clear the setVariable definitions.
	if utils.SliceContains(build.Migrations, PluralizeSetVariable) {
		c = clearSetVariables(c)
	} else {
		// Otherwise, run the migration.
		c = migrateSetVariableToSetVariables(c)
	}

	// Future migrations here.
	return c
}
