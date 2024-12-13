// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func (p *Packager) confirmAction(ctx context.Context, stage string, warnings []string, sbomViewFiles []string) bool {
	pterm.Println()
	message.HeaderInfof("📦 PACKAGE DEFINITION")
	l := logger.From(ctx)
	err := utils.ColorPrintYAML(p.cfg.Pkg, p.getPackageYAMLHints(stage), true)
	if err != nil {
		message.WarnErr(err, "unable to print yaml")
	}

	// Print any potential breaking changes (if this is a Deploy confirm) between this CLI version and the deployed init package
	if stage == config.ZarfDeployStage {
		if p.cfg.Pkg.IsSBOMAble() {
			// Print the location that the user can view the package SBOMs from
			message.HorizontalRule()
			message.Title("Software Bill of Materials", "an inventory of all software contained in this package")

			if len(sbomViewFiles) > 0 {
				cwd, _ := os.Getwd()
				link := pterm.FgLightCyan.Sprint(pterm.Bold.Sprint(filepath.Join(cwd, layout.SBOMDir, filepath.Base(sbomViewFiles[0]))))
				inspect := pterm.BgBlack.Sprint(pterm.FgWhite.Sprint(pterm.Bold.Sprintf("$ zarf package inspect %s", p.cfg.PkgOpts.PackageSource)))

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
				l.Info("this package has SBOMs available for review in a temporary directory", "directory", filepath.Join(cwd, layout.SBOMDir))
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
			l.Warn(warning)
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
	var confirm bool
	if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
		// User aborted or declined, cancel the action
		return false
	}

	return true
}

func (p *Packager) getPackageYAMLHints(stage string) map[string]string {
	hints := map[string]string{}

	if stage == config.ZarfDeployStage {
		for _, variable := range p.cfg.Pkg.Variables {
			value, present := p.cfg.PkgOpts.SetVariables[variable.Name]
			if !present {
				value = fmt.Sprintf("'%s' (default)", helpers.Truncate(variable.Default, 20, false))
			} else {
				value = fmt.Sprintf("'%s'", helpers.Truncate(value, 20, false))
			}
			if variable.Sensitive {
				value = "'**sanitized**'"
			}
			hints = utils.AddRootListHint(hints, "name", variable.Name, fmt.Sprintf("currently set to %s", value))
		}
	}

	hints = utils.AddRootHint(hints, "metadata", "information about this package\n")
	hints = utils.AddRootHint(hints, "build", "info about the machine, zarf version, and user that created this package\n")
	hints = utils.AddRootHint(hints, "components", "components selected for this operation")
	hints = utils.AddRootHint(hints, "constants", "static values set by the package author")
	hints = utils.AddRootHint(hints, "variables", "deployment-specific values that are set on each package deployment")

	return hints
}
