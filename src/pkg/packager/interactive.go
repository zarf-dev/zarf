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
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func (p *Packager) confirmAction(ctx context.Context, stage string, warnings []string, sbomViewFiles []string) bool {
	l := logger.From(ctx)
	err := utils.ColorPrintYAML(p.cfg.Pkg, p.getPackageYAMLHints(stage), true)
	if err != nil {
		l.Error("unable to print yaml", "stage", stage, "error", err)
	}

	// Print any potential breaking changes (if this is a Deploy confirm) between this CLI version and the deployed init package
	if stage == config.ZarfDeployStage {
		if p.cfg.Pkg.IsSBOMAble() {
			// Print the location that the user can view the package SBOMs from
			if len(sbomViewFiles) > 0 {
				cwd, _ := os.Getwd()
				l.Info("this package has SBOMs available for review in a temporary directory", "directory", filepath.Join(cwd, layout.SBOMDir))
			} else {
				l.Warn("this package does NOT contain an SBOM.  If you require an SBOM, please contact the creator of this package to request a version that includes an SBOM.",
					"name", p.cfg.Pkg.Metadata.Name)
			}
		}
	}

	if len(warnings) > 0 {
		for _, warning := range warnings {
			l.Warn(warning)
		}
	}
	// Display prompt if not auto-confirmed
	if config.CommonOptions.Confirm {
		return config.CommonOptions.Confirm
	}

	// Prompt the user for confirmation, on abort return false
	prompt := &survey.Confirm{
		Message: stage + " this Zarf package?",
	}
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
