// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sbom contains tools for generating SBOMs.
package sbom

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
)

// ViewSBOMFiles opens a browser to view the SBOM files and pauses for user input.
func ViewSBOMFiles(directory string) {
	sbomViewFiles, _ := filepath.Glob(filepath.Join(directory, "sbom-viewer-*"))

	if len(sbomViewFiles) > 0 {
		link := sbomViewFiles[0]
		msg := fmt.Sprintf("This package has %d images with software bill-of-materials (SBOM) included. If your browser did not open automatically you can copy and paste this file location into your browser address bar to view them: %s\n\n", len(sbomViewFiles), link)
		message.Note(msg)

		if err := exec.LaunchURL(link); err != nil {
			message.Debug(err)
		}

		// Use survey.Input to hang until user input
		var value string
		prompt := &survey.Input{
			Message: "Hit the 'enter' key when you are done viewing the SBOM files",
			Default: "",
		}
		_ = survey.AskOne(prompt, &value)
	} else {
		message.Note("There were no images with software bill-of-materials (SBOM) included.")
	}
}

// OutputSBOMFiles outputs the sbom files into a specified directory.
func OutputSBOMFiles(sourceDir, outputDir, packageName string) (string, error) {
	packagePath := filepath.Join(outputDir, packageName)

	if err := os.RemoveAll(packagePath); err != nil {
		return "", err
	}

	if err := utils.CreateDirectory(packagePath, 0700); err != nil {
		return "", err
	}

	return packagePath, utils.CreatePathAndCopy(sourceDir, packagePath)
}
