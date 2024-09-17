// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/creator"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func Create(ctx context.Context, createOpts types.ZarfCreateOptions) error {
	dir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	packagePaths := layout.New(dir)

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	err = os.Chdir(createOpts.BaseDir)
	if err != nil {
		return fmt.Errorf("unable to access directory %q: %w", createOpts.BaseDir, err)
	}

	pc := creator.NewPackageCreator(createOpts, cwd)
	err = helpers.CreatePathAndCopy(layout.ZarfYAML, packagePaths.ZarfYAML)
	if err != nil {
		return err
	}
	pkg, warnings, err := pc.LoadPackageDefinition(ctx, packagePaths)
	if err != nil {
		return err
	}

	if !confirmAction(config.ZarfCreateStage, warnings, pkg) {
		return fmt.Errorf("package creation canceled")
	}

	err = pc.Assemble(ctx, packagePaths, pkg.Components, pkg.Metadata.Architecture)
	if err != nil {
		return err
	}

	// cd back for output
	if err := os.Chdir(cwd); err != nil {
		return err
	}

	return pc.Output(ctx, packagePaths, &pkg)
}

func confirmAction(stage string, warnings []string, pkg v1alpha1.ZarfPackage) bool {
	pterm.Println()
	message.HeaderInfof("PACKAGE DEFINITION")
	utils.ColorPrintYAML(pkg, nil, true)

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
	var confirm bool
	if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
		// User aborted or declined, cancel the action
		return false
	}

	return true
}
