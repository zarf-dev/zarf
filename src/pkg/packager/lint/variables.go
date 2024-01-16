// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// This regex takes a line and parses the text before and after a discovered template: https://regex101.com/r/ilUxAz/1
var regexTemplateLine = regexp.MustCompile("###ZARF_[A-Z0-9_]+###")

const depWarning = "This Zarf Package uses a deprecated variable: '%s' changed to '%s'."

func (validator *Validator) addVarIfNotExists(vv validatorVar) {
	vv.name = getVariableNameFromZarfVar(vv.name)
	varExists := slices.ContainsFunc(validator.pkgVars, func(v validatorVar) bool {
		return v.name == vv.name
	})
	if !varExists {
		validator.pkgVars = append(validator.pkgVars, vv)
	}
}

// Potentially it is time to move the main function into packager
// this can have the package and get things with it
// Or I can keep moving things out of packager and make them more generic functions
func checkForUnusedVariables(validator *Validator, node *composer.Node) error {
	// There are at least three different scenarios I need to cover
	// 1. The variables are in the actions of the zarf chart
	// 2. The variables are in a helm chart in the component
	// 3. The variables are in a file brough in by zarf
	// Initial idea is to go through each of these and as a variable is found, take it out of the list
	// At the end we warn that whatever is still in the list is unused.
	// We will also want to do this with both zarf const and zarf var
	// Where / how are constant variables set?

	for _, file := range node.ZarfComponent.Files {

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		fileLocation := filepath.Join(cwd, node.ImportLocation(), file.Source)

		fileList := []string{}
		if utils.IsDir(fileLocation) {
			files, _ := utils.RecursiveFileList(fileLocation, nil, false)
			fileList = append(fileList, files...)
		} else {
			fileList = append(fileList, fileLocation)
		}

		for _, subFile := range fileList {
			// Check if the file looks like a text file
			isText, err := utils.IsTextFile(subFile)
			if err != nil {
				message.Debugf("unable to determine if file %s is a text file: %s", subFile, err)
			}

			if isText {
				if err := checkFileForVar(validator, subFile, node.ImportLocation()); err != nil {
					return fmt.Errorf("unable to template file %s: %w", subFile, err)
				}
			}
		}
	}
	// The data I need here are the helm files. This includes basically every file that is packaged with hel

	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	loaded := layout.New(tmp)

	componentPaths, err := loaded.Components.Create(node.ZarfComponent)
	if err != nil {
		return err
	}

	composer.FixPaths(&node.ZarfComponent, node.ImportLocation())

	for _, chart := range node.ZarfComponent.Charts {
		helmCfg := helm.New(
			chart,
			componentPaths.Charts,
			componentPaths.Values,
		)

		err := helmCfg.PackageChart(node.ZarfComponent.DeprecatedCosignKeyPath)
		if err != nil {
			return fmt.Errorf("in lint: unable to package the chart %s: %s", chart.URL, err.Error())
		}

		// Generate helm templates for this chart
		template, _, err := helmCfg.TemplateChart()
		// TODO aabro, when a chart breaks at templating should I skip the error?
		if err != nil {
			validator.addWarning(validatorMessage{
				description:    fmt.Sprintf("unable to template chart %q: %s", chart.URL, err.Error()),
				packageRelPath: node.ImportLocation()})
			continue
		}
		chartPath := filepath.Join(tmp + "chart.yaml")
		utils.WriteFile(chartPath, []byte(template))
		if err := checkFileForVar(validator, chartPath, node.ImportLocation()); err != nil {
			return fmt.Errorf("unable to template file %s: %w", chartPath, err)
		}

	}

	// Ultimately the only thing we need is a way to scan each yaml file in the helm chart.
	// If I was to do it the exact same way as we do it during deploy I would need to package it into a single chart file
	// Ultimately I need to figure out how we get the skeleton down. For now, I should just pull from a local chart

	return nil
}

func checkFileForVar(validator *Validator, filepath, pkgRelPath string) error {
	textFile, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer textFile.Close()

	fileScanner := bufio.NewScanner(textFile)

	// Set the buffer to 1 MiB to handle long lines (i.e. base64 text in a secret)
	// 1 MiB is around the documented maximum size for secrets and configmaps
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	fileScanner.Buffer(buf, maxCapacity)

	// Set the scanner to split on new lines
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		findVarsInLine(validator, fileScanner.Text(), pkgRelPath)
	}
	return nil
}

func findVarsInLine(validator *Validator, line, pkgRelPath string) {
	deprecations := template.GetTemplateDeprecations()
	matches := regexTemplateLine.FindAllString(line, -1)

	for _, templateKey := range matches {
		_, present := deprecations[templateKey]
		if present {
			depWarning := fmt.Sprintf(depWarning, templateKey, deprecations[templateKey])
			validator.addWarning(validatorMessage{description: depWarning, packageRelPath: pkgRelPath})
		}
		if strings.HasPrefix(templateKey, "###ZARF_CONST_") || strings.HasPrefix(templateKey, "###ZARF_VAR_") {
			varName := getVariableNameFromZarfVar(templateKey)
			validator.addVarIfNotExists(validatorVar{name: varName, relativePath: pkgRelPath, usedByPackage: true})

			for i := range validator.pkgVars {
				if validator.pkgVars[i].name == varName {
					validator.pkgVars[i].usedByPackage = true
				}
			}
		}
	}
}
