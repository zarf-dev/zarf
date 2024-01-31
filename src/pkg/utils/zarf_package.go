// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// IsInitConfig returns whether the provided Zarf package is an init config.
func IsInitConfig(pkg types.ZarfPackage) bool {
	return pkg.Kind == types.ZarfInitConfig
}

// GetInitPackageName returns the formatted name of the init package.
func GetInitPackageName(arch string) string {
	if arch == "" {
		// No package has been loaded yet so lookup GetArch() with no package info
		arch = config.GetArch()
	}
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
}

// GetPackageName returns the formatted name of the package.
func GetPackageName(pkg types.ZarfPackage, diffData types.DifferentialData) string {
	if IsInitConfig(pkg) {
		return GetInitPackageName(pkg.Metadata.Architecture)
	}

	packageName := pkg.Metadata.Name
	suffix := "tar.zst"
	if pkg.Metadata.Uncompressed {
		suffix = "tar"
	}

	packageFileName := fmt.Sprintf("%s%s-%s", config.ZarfPackagePrefix, packageName, pkg.Metadata.Architecture)
	if pkg.Build.Differential {
		packageFileName = fmt.Sprintf("%s-%s-differential-%s", packageFileName, diffData.DifferentialPackageVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		packageFileName = fmt.Sprintf("%s-%s", packageFileName, pkg.Metadata.Version)
	}

	return fmt.Sprintf("%s.%s", packageFileName, suffix)
}

// IsSBOMAble checks if a package has contents that an SBOM can be created on (i.e. images, files, or data injections)
func IsSBOMAble(pkg types.ZarfPackage) bool {
	for _, c := range pkg.Components {
		if len(c.Images) > 0 || len(c.Files) > 0 || len(c.DataInjections) > 0 {
			return true
		}
	}

	return false
}

func ConfirmAction(stage, sbomDir string, sbomViewFiles, warnings []string, pkg types.ZarfPackage, pkgOpts types.ZarfPackageOptions) (confirm bool) {

	pterm.Println()
	message.HeaderInfof("📦 PACKAGE DEFINITION")
	ColorPrintYAML(pkg, getPackageYAMLHints(stage, pkg.Variables, pkgOpts.SetVariables), true)

	// Print any potential breaking changes (if this is a Deploy confirm) between this CLI version and the deployed init package
	if stage == config.ZarfDeployStage {
		if IsSBOMAble(pkg) {
			// Print the location that the user can view the package SBOMs from
			message.HorizontalRule()
			message.Title("Software Bill of Materials", "an inventory of all software contained in this package")

			if len(sbomViewFiles) > 0 {
				cwd, _ := os.Getwd()
				link := pterm.FgLightCyan.Sprint(pterm.Bold.Sprint(filepath.Join(cwd, sbomDir, filepath.Base(sbomViewFiles[0]))))
				inspect := pterm.BgBlack.Sprint(pterm.FgWhite.Sprint(pterm.Bold.Sprintf("$ zarf package inspect %s", pkgOpts.PackageSource)))

				artifactMsg := pterm.Bold.Sprintf("%d artifacts", len(sbomViewFiles)) + " to be reviewed. These are"
				if len(sbomViewFiles) == 1 {
					artifactMsg = pterm.Bold.Sprintf("%d artifact", len(sbomViewFiles)) + " to be reviewed. This is"
				}

				msg := fmt.Sprintf("This package has %s available in a temporary '%s' folder in this directory and will be removed upon deployment.\n", artifactMsg, pterm.Bold.Sprint("zarf-sbom"))
				viewNow := fmt.Sprintf("\n- View SBOMs %s by navigating to the '%s' folder or copying this link into a browser:\n%s", pterm.Bold.Sprint("now"), pterm.Bold.Sprint("zarf-sbom"), link)
				viewLater := fmt.Sprintf("\n- View SBOMs %s deployment with this command:\n%s", pterm.Bold.Sprint("after"), inspect)

				message.Note(msg)
				pterm.Println(viewNow)
				pterm.Println(viewLater)
			} else {
				message.Warn("This package does NOT contain an SBOM.  If you require an SBOM, please contact the creator of this package to request a version that includes an SBOM.")
			}
		}
	}

	if len(warnings) > 0 {
		message.HorizontalRule()
		message.Title("Package Warnings", "the following warnings were flagged while reading the package")
		for _, warning := range warnings {
			message.Warn(warning)
		}
	}

	message.HorizontalRule()

	// Display prompt if not auto-confirmed
	if config.CommonOptions.Confirm {
		pterm.Println()
		message.Successf("%s Zarf package confirmed", stage)
		return config.CommonOptions.Confirm
	}

	prompt := &survey.Confirm{
		Message: stage + " this Zarf package?",
	}

	pterm.Println()

	// Prompt the user for confirmation, on abort return false
	if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
		// User aborted or declined, cancel the action
		return false
	}

	return true
}

func getPackageYAMLHints(stage string, pkgVars []types.ZarfPackageVariable, setVars map[string]string) map[string]string {
	hints := map[string]string{}

	if stage == config.ZarfDeployStage {
		for _, variable := range pkgVars {
			value, present := setVars[variable.Name]
			if !present {
				value = fmt.Sprintf("'%s' (default)", message.Truncate(variable.Default, 20, false))
			} else {
				value = fmt.Sprintf("'%s'", message.Truncate(value, 20, false))
			}
			if variable.Sensitive {
				value = "'**sanitized**'"
			}
			hints = AddRootListHint(hints, "name", variable.Name, fmt.Sprintf("currently set to %s", value))
		}
	}

	hints = AddRootHint(hints, "metadata", "information about this package\n")
	hints = AddRootHint(hints, "build", "info about the machine, zarf version, and user that created this package\n")
	hints = AddRootHint(hints, "components", "definition of capabilities this package deploys")
	hints = AddRootHint(hints, "constants", "static values set by the package author")
	hints = AddRootHint(hints, "variables", "deployment-specific values that are set on each package deployment")

	return hints
}
