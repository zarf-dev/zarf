// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs.
package sbom

import (
	"context"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/AlecAivazis/survey/v2"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

// ViewSBOMFiles opens a browser to view the SBOM files and pauses for user input.
func ViewSBOMFiles(ctx context.Context, directory string) error {
	l := logger.From(ctx)
	sbomViewFiles, err := filepath.Glob(filepath.Join(directory, "sbom-viewer-*"))
	if err != nil {
		return err
	}

	if len(sbomViewFiles) == 0 {
		l.Info("there were no images with software bill-of-materials (SBOM) included.")
		return nil
	}

	link := sbomViewFiles[0]
	l.Info("this package has images with software bill-of-materials (SBOM) included. If your browser did not open automatically you can copy and paste this file location into your browser address bar to view them", "SBOMCount", len(sbomViewFiles), "link", link)
	if err := exec.LaunchURL(link); err != nil {
		return err
	}
	var value string
	prompt := &survey.Input{
		Message: "Hit the 'enter' key when you are done viewing the SBOM files",
		Default: "",
	}
	err = survey.AskOne(prompt, &value)
	if err != nil {
		return err
	}
	return nil
}
