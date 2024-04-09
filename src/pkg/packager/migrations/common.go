// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package migrations handles component deprecations and package migrations
package migrations

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// breakingChange represents a breaking change that happened on a specified Zarf version
type breakingChange struct {
	version    *semver.Version
	title      string
	mitigation string
}

// LastNonBreakingVersion is the last version that did not have any breaking changes
//
// This should be updated when a breaking change is introduced to the Zarf package structure.  See: https://github.com/defenseunicorns/zarf/releases/tag/v0.32.2
const LastNonBreakingVersion = "v0.27.0"

// List of breaking changes to warn the user of.
var breakingChanges = []breakingChange{
	{
		version:    semver.New(0, 26, 0, "", ""),
		title:      "Zarf container images are now mutated based on tag instead of repository name.",
		mitigation: "Reinitialize the cluster using v0.26.0 or later and redeploy existing packages to update the image references (you can view existing packages with 'zarf package list' and view cluster images with 'zarf tools registry catalog').",
	},
}

// DeprecatedComponentMigration represents a migration that can be run on a component.
//
// DeprecatedComponentMigrations are migrations that seamlessly migrate deprecated component definitions.
type DeprecatedComponentMigration interface {
	fmt.Stringer
	// Run runs the migration on the component
	Run(c types.ZarfComponent) (types.ZarfComponent, string)
	// Clear clears the deprecated configuration from the component
	Clear(mc types.ZarfComponent) types.ZarfComponent
}

// DeprecatedComponentMigrations returns a list of all current deprecated component-level migrations.
func DeprecatedComponentMigrations() []DeprecatedComponentMigration {
	return []DeprecatedComponentMigration{
		ScriptsToActions{},
		SetVariableToSetVariables{},
	}
}

// FeatureMigration represents a feature migration that can be run on a package.
//
// Every migration is mapped to a specific feature, and the feature's identifier is added to the package metadata.
type FeatureMigration interface {
	fmt.Stringer
	// Run runs the feature migration on the package
	Run(pkg types.ZarfPackage) types.ZarfPackage
}

// FeatureMigrations returns a list of all current feature migrations.
func FeatureMigrations() []FeatureMigration {
	return []FeatureMigration{
		DefaultRequired{},
	}
}

// PrintBreakingChanges prints the breaking changes between the provided version and the current CLIVersion
func PrintBreakingChanges(deployedZarfVersion string) {
	deployedSemver, err := semver.NewVersion(deployedZarfVersion)
	if err != nil {
		message.Debugf("Unable to check for breaking changes between Zarf versions")
		return
	}

	applicableBreakingChanges := []breakingChange{}

	// Calculate the applicable breaking changes
	for _, breakingChange := range breakingChanges {
		if deployedSemver.LessThan(breakingChange.version) {
			applicableBreakingChanges = append(applicableBreakingChanges, breakingChange)
		}
	}

	if len(applicableBreakingChanges) > 0 {
		// Print header information
		message.HorizontalRule()
		message.Title("Potential Breaking Changes", "breaking changes that may cause issues with this package")

		// Print information about the versions
		format := pterm.FgYellow.Sprint("CLI version ") + "%s" + pterm.FgYellow.Sprint(" is being used to deploy to a cluster that was initialized with ") +
			"%s" + pterm.FgYellow.Sprint(". Between these versions there are the following breaking changes to consider:")
		cliVersion := pterm.Bold.Sprintf(config.CLIVersion)
		deployedVersion := pterm.Bold.Sprintf(deployedZarfVersion)
		message.Warnf(format, cliVersion, deployedVersion)

		// Print each applicable breaking change
		for idx, applicableBreakingChange := range applicableBreakingChanges {
			titleFormat := pterm.Bold.Sprintf("\n %d. ", idx+1) + "%s"

			pterm.Printfln(titleFormat, applicableBreakingChange.title)

			mitigationText := message.Paragraphn(96, "%s", pterm.FgLightCyan.Sprint(applicableBreakingChange.mitigation))

			pterm.Printfln("\n  - %s", pterm.Bold.Sprint("Mitigation:"))
			pterm.Printfln("    %s", strings.ReplaceAll(mitigationText, "\n", "\n    "))
		}

		message.HorizontalRule()
	}
}
