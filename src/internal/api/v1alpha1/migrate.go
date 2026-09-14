// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"context"
	"fmt"
	"slices"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// List of migrations tracked in the zarf.yaml build data.
const (
	ScriptsToActionsMigrated = "scripts-to-actions"
	PluralizeSetVariable     = "pluralize-set-variable"

	legacyScriptsMaxRetries = 10000
)

// ApplyMigrations applies all v1alpha1 schema migrations to pkg, logging any warnings to ctx.
func ApplyMigrations(ctx context.Context, pkg v1alpha1.ZarfPackage) v1alpha1.ZarfPackage {
	pkg, warnings := migrateDeprecated(pkg)
	for _, warning := range warnings {
		logger.From(ctx).Warn(warning)
	}
	return pkg
}

func migrateDeprecated(pkg v1alpha1.ZarfPackage) (v1alpha1.ZarfPackage, []string) {
	warnings := []string{}

	if pkg.Metadata.YOLO {
		warnings = append(warnings, "metadata.yolo is deprecated and will be removed in the next schema version. Use --connected when running zarf package deploy instead.")
	}

	migratedComponents := []v1alpha1.ZarfComponent{}
	for _, comp := range pkg.Components {
		if slices.Contains(pkg.Build.Migrations, ScriptsToActionsMigrated) {
			comp.DeprecatedScripts = v1alpha1.DeprecatedZarfComponentScripts{}
		} else {
			var warning string
			if comp, warning = migrateScriptsToActions(comp); warning != "" {
				warnings = append(warnings, warning)
			}
		}

		if slices.Contains(pkg.Build.Migrations, PluralizeSetVariable) {
			comp = clearSetVariables(comp)
		} else {
			var warning string
			if comp, warning = migrateSetVariableToSetVariables(comp); warning != "" {
				warnings = append(warnings, warning)
			}
		}

		// Show a warning if the component contains a group as that has been deprecated and will be removed.
		if comp.DeprecatedGroup != "" {
			warnings = append(warnings, fmt.Sprintf("Component %s is using group which has been deprecated and will be removed in the next schema version. Please migrate to another solution.", comp.Name))
		}

		if len(comp.DataInjections) != 0 {
			warnings = append(warnings, fmt.Sprintf("Component %s is using data injections which has been deprecated and will be removed in the next schema version. Please migrate to another solution.", comp.Name))
		}

		migratedComponents = append(migratedComponents, comp)
	}
	pkg.Components = migratedComponents

	// Record the migrations that have been run on the package.
	pkg.Build.Migrations = []string{
		ScriptsToActionsMigrated,
		PluralizeSetVariable,
	}

	return pkg, warnings
}

// migrateScriptsToActions converts the deprecated scripts to the new actions
// The following have no migration:
// - Actions.Create.After
// - Actions.Remove.*
// - Actions.*.OnSuccess
// - Actions.*.OnFailure
// - Actions.*.*.Env
func migrateScriptsToActions(c v1alpha1.ZarfComponent) (v1alpha1.ZarfComponent, string) {
	var hasScripts bool

	// Convert a script configs to action defaults.
	defaults := v1alpha1.ZarfComponentActionDefaults{
		// ShowOutput (default false) -> Mute (default false)
		Mute: !c.DeprecatedScripts.ShowOutput,
		// TimeoutSeconds -> MaxSeconds
		MaxTotalSeconds: c.DeprecatedScripts.TimeoutSeconds,
	}

	// Retry is now an integer vs a boolean (implicit infinite retries), so cap the migrated value.
	if c.DeprecatedScripts.Retry {
		defaults.MaxRetries = legacyScriptsMaxRetries
	}

	// Scripts.Prepare -> Actions.Create.Before
	if len(c.DeprecatedScripts.Prepare) > 0 {
		hasScripts = true
		c.Actions.OnCreate.Defaults = defaults
		for _, s := range c.DeprecatedScripts.Prepare {
			c.Actions.OnCreate.Before = append(c.Actions.OnCreate.Before, v1alpha1.ZarfComponentAction{Cmd: s})
		}
	}

	// Scripts.Before -> Actions.Deploy.Before
	if len(c.DeprecatedScripts.Before) > 0 {
		hasScripts = true
		c.Actions.OnDeploy.Defaults = defaults
		for _, s := range c.DeprecatedScripts.Before {
			c.Actions.OnDeploy.Before = append(c.Actions.OnDeploy.Before, v1alpha1.ZarfComponentAction{Cmd: s})
		}
	}

	// Scripts.After -> Actions.Deploy.After
	if len(c.DeprecatedScripts.After) > 0 {
		hasScripts = true
		c.Actions.OnDeploy.Defaults = defaults
		for _, s := range c.DeprecatedScripts.After {
			c.Actions.OnDeploy.After = append(c.Actions.OnDeploy.After, v1alpha1.ZarfComponentAction{Cmd: s})
		}
	}

	// Leave deprecated scripts in place, but warn users
	if hasScripts {
		return c, fmt.Sprintf("Component '%s' is using scripts which will be removed in the next schema version. Please migrate to actions.", c.Name)
	}

	return c, ""
}

func migrateSetVariableToSetVariables(c v1alpha1.ZarfComponent) (v1alpha1.ZarfComponent, string) {
	hasSetVariable := false

	// Migrate OnCreate SetVariables
	c.Actions.OnCreate.After, hasSetVariable = migrateComponentActions(c.Actions.OnCreate.After, hasSetVariable)
	c.Actions.OnCreate.Before, hasSetVariable = migrateComponentActions(c.Actions.OnCreate.Before, hasSetVariable)
	c.Actions.OnCreate.OnSuccess, hasSetVariable = migrateComponentActions(c.Actions.OnCreate.OnSuccess, hasSetVariable)
	c.Actions.OnCreate.OnFailure, hasSetVariable = migrateComponentActions(c.Actions.OnCreate.OnFailure, hasSetVariable)

	// Migrate OnDeploy SetVariables
	c.Actions.OnDeploy.After, hasSetVariable = migrateComponentActions(c.Actions.OnDeploy.After, hasSetVariable)
	c.Actions.OnDeploy.Before, hasSetVariable = migrateComponentActions(c.Actions.OnDeploy.Before, hasSetVariable)
	c.Actions.OnDeploy.OnSuccess, hasSetVariable = migrateComponentActions(c.Actions.OnDeploy.OnSuccess, hasSetVariable)
	c.Actions.OnDeploy.OnFailure, hasSetVariable = migrateComponentActions(c.Actions.OnDeploy.OnFailure, hasSetVariable)

	// Migrate OnRemove SetVariables
	c.Actions.OnRemove.After, hasSetVariable = migrateComponentActions(c.Actions.OnRemove.After, hasSetVariable)
	c.Actions.OnRemove.Before, hasSetVariable = migrateComponentActions(c.Actions.OnRemove.Before, hasSetVariable)
	c.Actions.OnRemove.OnSuccess, hasSetVariable = migrateComponentActions(c.Actions.OnRemove.OnSuccess, hasSetVariable)
	c.Actions.OnRemove.OnFailure, hasSetVariable = migrateComponentActions(c.Actions.OnRemove.OnFailure, hasSetVariable)

	// Leave deprecated setVariable in place, but warn users
	if hasSetVariable {
		return c, fmt.Sprintf("Component '%s' is using setVariable in actions which will be removed in the next schema version. Please migrate to the list form of setVariables.", c.Name)
	}

	return c, ""
}

func migrateComponentActions(actions []v1alpha1.ZarfComponentAction, hasSetVariable bool) ([]v1alpha1.ZarfComponentAction, bool) {
	for i := range actions {
		var migrated bool
		actions[i], migrated = migrateComponentAction(actions[i])
		hasSetVariable = hasSetVariable || migrated
	}

	return actions, hasSetVariable
}

func migrateComponentAction(action v1alpha1.ZarfComponentAction) (v1alpha1.ZarfComponentAction, bool) {
	if action.DeprecatedSetVariable == "" || len(action.SetVariables) > 0 {
		return action, false
	}

	action.SetVariables = []v1alpha1.Variable{
		{
			Name:      action.DeprecatedSetVariable,
			Sensitive: false,
		},
	}
	return action, true
}

func clearSetVariables(c v1alpha1.ZarfComponent) v1alpha1.ZarfComponent {
	clearVar := func(actions []v1alpha1.ZarfComponentAction) []v1alpha1.ZarfComponentAction {
		for i := range actions {
			actions[i].DeprecatedSetVariable = ""
		}

		return actions
	}

	// Clear OnCreate SetVariables
	c.Actions.OnCreate.After = clearVar(c.Actions.OnCreate.After)
	c.Actions.OnCreate.Before = clearVar(c.Actions.OnCreate.Before)
	c.Actions.OnCreate.OnSuccess = clearVar(c.Actions.OnCreate.OnSuccess)
	c.Actions.OnCreate.OnFailure = clearVar(c.Actions.OnCreate.OnFailure)

	// Clear OnDeploy SetVariables
	c.Actions.OnDeploy.After = clearVar(c.Actions.OnDeploy.After)
	c.Actions.OnDeploy.Before = clearVar(c.Actions.OnDeploy.Before)
	c.Actions.OnDeploy.OnSuccess = clearVar(c.Actions.OnDeploy.OnSuccess)
	c.Actions.OnDeploy.OnFailure = clearVar(c.Actions.OnDeploy.OnFailure)

	// Clear OnRemove SetVariables
	c.Actions.OnRemove.After = clearVar(c.Actions.OnRemove.After)
	c.Actions.OnRemove.Before = clearVar(c.Actions.OnRemove.Before)
	c.Actions.OnRemove.OnSuccess = clearVar(c.Actions.OnRemove.OnSuccess)
	c.Actions.OnRemove.OnFailure = clearVar(c.Actions.OnRemove.OnFailure)

	return c
}
