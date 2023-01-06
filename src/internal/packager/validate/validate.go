// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validate provides Zarf package validation functions.
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Run performs config validations.
func Run(pkg types.ZarfPackage) error {
	if pkg.Kind == "ZarfInitConfig" && pkg.Metadata.YOLO {
		return fmt.Errorf(lang.PkgValidateErrInitNoYOLO)
	}

	if err := validatePackageName(pkg.Metadata.Name); err != nil {
		return fmt.Errorf(lang.PkgValidateErrName, err)
	}

	for _, variable := range pkg.Variables {
		if err := validatePackageVariable(variable); err != nil {
			return fmt.Errorf(lang.PkgValidateErrVariable, err)
		}
	}

	for _, constant := range pkg.Constants {
		if err := validatePackageConstant(constant); err != nil {
			return fmt.Errorf(lang.PkgValidateErrConstant, err)
		}
	}

	uniqueNames := make(map[string]bool)

	for _, component := range pkg.Components {
		// ensure component name is unique
		if _, ok := uniqueNames[component.Name]; ok {
			return fmt.Errorf(lang.PkgValidateErrCompenantNameNotUnique, component.Name)
		}
		uniqueNames[component.Name] = true

		if err := validateComponent(pkg, component); err != nil {
			return fmt.Errorf(lang.PkgValidateErrComponent, err)
		}
	}

	return nil
}

// ImportPackage validates the package trying to be imported.
func ImportPackage(composedComponent *types.ZarfComponent) error {
	path := composedComponent.Import.Path

	// ensure path exists
	if !(len(path) > 0) {
		return fmt.Errorf(lang.PkgValidateErrImportPathMissing, composedComponent.Name)
	}

	// remove zarf.yaml from path if path has zarf.yaml suffix
	if strings.HasSuffix(path, config.ZarfYAML) {
		path = strings.Split(path, config.ZarfYAML)[0]
	}

	// add a forward slash to end of path if it does not have one
	if !strings.HasSuffix(path, "/") {
		path = filepath.Clean(path) + string(os.PathSeparator)
	}

	// ensure there is a zarf.yaml in provided path
	if utils.InvalidPath(path + config.ZarfYAML) {
		return fmt.Errorf(lang.PkgValidateErrImportPathInvalid, composedComponent.Import.Path)
	}

	return nil
}

func oneIfNotEmpty(testString string) int {
	if testString == "" {
		return 0
	}

	return 1
}

func validateComponent(pkg types.ZarfPackage, component types.ZarfComponent) error {
	if component.Required {
		if component.Default {
			return fmt.Errorf(lang.PkgValidateErrComponentReqDefault, component.Name)
		}
		if component.Group != "" {
			return fmt.Errorf(lang.PkgValidateErrComponentReqGrouped, component.Name)
		}
	}

	for _, chart := range component.Charts {
		if err := validateChart(chart); err != nil {
			return fmt.Errorf(lang.PkgValidateErrChart, err)
		}
	}

	for _, manifest := range component.Manifests {
		if err := validateManifest(manifest); err != nil {
			return fmt.Errorf(lang.PkgValidateErrManifest, err)
		}
	}

	if pkg.Metadata.YOLO {
		if err := validateYOLO(component); err != nil {
			return fmt.Errorf(lang.PkgValidateErrComponentYOLO, component.Name, err)
		}
	}

	return nil
}

func validateYOLO(component types.ZarfComponent) error {
	if len(component.Images) > 0 {
		return fmt.Errorf(lang.PkgValidateErrYOLONoOCI)
	}

	if len(component.Repos) > 0 {
		return fmt.Errorf(lang.PkgValidateErrYOLONoGit)
	}

	if component.Only.Cluster.Architecture != "" {
		return fmt.Errorf(lang.PkgValidateErrYOLONoArch)
	}

	if len(component.Only.Cluster.Distros) > 0 {
		return fmt.Errorf(lang.PkgValidateErrYOLONoDistro)
	}

	return nil
}

func validatePackageName(subject string) error {
	// https://regex101.com/r/vpi8a8/1
	isValid := regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString

	if !isValid(subject) {
		return fmt.Errorf(lang.PkgValidateErrPkgName, subject)
	}

	return nil
}

func validatePackageVariable(subject types.ZarfPackageVariable) error {
	isAllCapsUnderscore := regexp.MustCompile(`^[A-Z_]+$`).MatchString

	// ensure the variable name is only capitals and underscores
	if !isAllCapsUnderscore(subject.Name) {
		return fmt.Errorf(lang.PkgValidateErrPkgVariableName, subject.Name)
	}

	return nil
}

func validatePackageConstant(subject types.ZarfPackageConstant) error {
	isAllCapsUnderscore := regexp.MustCompile(`^[A-Z_]+$`).MatchString

	// ensure the constant name is only capitals and underscores
	if !isAllCapsUnderscore(subject.Name) {
		return fmt.Errorf(lang.PkgValidateErrPkgConstantName, subject.Name)
	}

	return nil
}

func validateChart(chart types.ZarfChart) error {
	// Don't allow empty names
	if chart.Name == "" {
		return fmt.Errorf(lang.PkgValidateErrChartNameMissing, chart.Name)
	}

	// Helm max release name
	if len(chart.Name) > config.ZarfMaxChartNameLength {
		return fmt.Errorf(lang.PkgValidateErrChartName, chart.Name, config.ZarfMaxChartNameLength)
	}

	// Must have a namespace
	if chart.Namespace == "" {
		return fmt.Errorf(lang.PkgValidateErrChartNamespaceMissing, chart.Name)
	}

	// Must only have a url or localPath
	count := oneIfNotEmpty(chart.URL) + oneIfNotEmpty(chart.LocalPath)
	if count != 1 {
		return fmt.Errorf(lang.PkgValidateErrChartURLOrPath, chart.Name)
	}

	// Must have a version
	if chart.Version == "" {
		return fmt.Errorf(lang.PkgValidateErrChartVersion, chart.Name)
	}

	return nil
}

func validateManifest(manifest types.ZarfManifest) error {
	// Don't allow empty names
	if manifest.Name == "" {
		return fmt.Errorf(lang.PkgValidateErrManifestNameMissing, manifest.Name)
	}

	// Helm max release name
	if len(manifest.Name) > config.ZarfMaxChartNameLength {
		return fmt.Errorf(lang.PkgValidateErrManifestNameLength, manifest.Name, config.ZarfMaxChartNameLength)
	}

	// Require files in manifest
	if len(manifest.Files) < 1 && len(manifest.Kustomizations) < 1 {
		return fmt.Errorf(lang.PkgValidateErrManifestFileOrKustomize, manifest.Name)
	}

	return nil
}
