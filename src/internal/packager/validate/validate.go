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
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// https://regex101.com/r/vpi8a8/1
	isLowercaseNumberHyphen     = regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString
	isUppercaseNumberUnderscore = regexp.MustCompile(`^[A-Z0-9_]+$`).MatchString
)

// Run performs config validations.
func Run(pkg types.ZarfPackage) error {
	if pkg.Kind == types.ZarfInitConfig && pkg.Metadata.YOLO {
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

	uniqueComponentNames := make(map[string]bool)

	for _, component := range pkg.Components {
		// ensure component name is unique
		if _, ok := uniqueComponentNames[component.Name]; ok {
			return fmt.Errorf(lang.PkgValidateErrComponentNameNotUnique, component.Name)
		}
		uniqueComponentNames[component.Name] = true

		if err := validateComponent(pkg, component); err != nil {
			return fmt.Errorf(lang.PkgValidateErrComponent, err)
		}
	}

	return nil
}

// ImportPackage validates the package trying to be imported.
func ImportPackage(composedComponent *types.ZarfComponent) error {
	path := composedComponent.Import.Path
	url := composedComponent.Import.URL

	if url == "" {
		// ensure path exists
		if path == "" {
			return fmt.Errorf(lang.PkgValidateErrImportPathMissing, composedComponent.Name)
		}

		// remove zarf.yaml from path if path has zarf.yaml suffix
		if strings.HasSuffix(path, config.ZarfYAML) {
			path = strings.Split(path, config.ZarfYAML)[0]
		}

		// add a forward slash to end of path if it does not have one
		if !strings.HasSuffix(path, string(os.PathSeparator)) {
			path = filepath.Clean(path) + string(os.PathSeparator)
		}

		// ensure there is a zarf.yaml in provided path
		if utils.InvalidPath(filepath.Join(path, config.ZarfYAML)) {
			return fmt.Errorf(lang.PkgValidateErrImportPathInvalid, composedComponent.Import.Path)
		}
	} else {
		// ensure path is empty
		if path != "" {
			return fmt.Errorf(lang.PkgValidateErrImportOptions, composedComponent.Name)
		}
		ok := helpers.IsOCIURL(url)
		if !ok {
			return fmt.Errorf(lang.PkgValidateErrImportURLInvalid, composedComponent.Import.URL)
		}
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

	uniqueChartNames := make(map[string]bool)
	for _, chart := range component.Charts {
		// ensure chart name is unique
		if _, ok := uniqueChartNames[chart.Name]; ok {
			return fmt.Errorf(lang.PkgValidateErrChartNameNotUnique, chart.Name)
		}
		uniqueChartNames[chart.Name] = true

		if err := validateChart(chart); err != nil {
			return fmt.Errorf(lang.PkgValidateErrChart, err)
		}
	}

	uniqueManifestNames := make(map[string]bool)
	for _, manifest := range component.Manifests {
		// ensure manifest name is unique
		if _, ok := uniqueManifestNames[manifest.Name]; ok {
			return fmt.Errorf(lang.PkgValidateErrManifestNameNotUnique, manifest.Name)
		}
		uniqueManifestNames[manifest.Name] = true

		if err := validateManifest(manifest); err != nil {
			return fmt.Errorf(lang.PkgValidateErrManifest, err)
		}
	}

	if pkg.Metadata.YOLO {
		if err := validateYOLO(component); err != nil {
			return fmt.Errorf(lang.PkgValidateErrComponentYOLO, component.Name, err)
		}
	}

	if containsVariables, err := validateActionset(component.Actions.OnCreate); err != nil {
		return fmt.Errorf(lang.PkgValidateErrAction, err)
	} else if containsVariables {
		return fmt.Errorf(lang.PkgValidateErrActionVariables, component.Name)
	}

	if _, err := validateActionset(component.Actions.OnDeploy); err != nil {
		return fmt.Errorf(lang.PkgValidateErrAction, err)
	}

	if containsVariables, err := validateActionset(component.Actions.OnRemove); err != nil {
		return fmt.Errorf(lang.PkgValidateErrAction, err)
	} else if containsVariables {
		return fmt.Errorf(lang.PkgValidateErrActionVariables, component.Name)
	}

	return nil
}

func validateActionset(actions types.ZarfComponentActionSet) (bool, error) {
	containsVariables := false

	validate := func(actions []types.ZarfComponentAction) error {
		for _, action := range actions {
			if cv, err := validateAction(action); err != nil {
				return err
			} else if cv {
				containsVariables = true
			}
		}

		return nil
	}

	if err := validate(actions.Before); err != nil {
		return containsVariables, err
	}
	if err := validate(actions.After); err != nil {
		return containsVariables, err
	}
	if err := validate(actions.OnSuccess); err != nil {
		return containsVariables, err
	}
	if err := validate(actions.OnFailure); err != nil {
		return containsVariables, err
	}

	return containsVariables, nil
}

func validateAction(action types.ZarfComponentAction) (bool, error) {
	containsVariables := false

	// Validate SetVariable
	for _, variable := range action.SetVariables {
		if !isUppercaseNumberUnderscore(variable.Name) {
			return containsVariables, fmt.Errorf(lang.PkgValidateMustBeUppercase, variable.Name)
		}
		containsVariables = true
	}

	if action.Wait != nil {
		// Validate only cmd or wait, not both
		if action.Cmd != "" {
			return containsVariables, fmt.Errorf(lang.PkgValidateErrActionCmdWait, action.Cmd)
		}

		// Validate only cluster or network, not both
		if action.Wait.Cluster != nil && action.Wait.Network != nil {
			return containsVariables, fmt.Errorf(lang.PkgValidateErrActionClusterNetwork)
		}

		// Validate at least one of cluster or network
		if action.Wait.Cluster == nil && action.Wait.Network == nil {
			return containsVariables, fmt.Errorf(lang.PkgValidateErrActionClusterNetwork)
		}
	}

	return containsVariables, nil
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
	if !isLowercaseNumberHyphen(subject) {
		return fmt.Errorf(lang.PkgValidateErrPkgName, subject)
	}

	return nil
}

func validatePackageVariable(subject types.ZarfPackageVariable) error {
	// ensure the variable name is only capitals and underscores
	if !isUppercaseNumberUnderscore(subject.Name) {
		return fmt.Errorf(lang.PkgValidateMustBeUppercase, subject.Name)
	}

	return nil
}

func validatePackageConstant(subject types.ZarfPackageConstant) error {
	// ensure the constant name is only capitals and underscores
	if !isUppercaseNumberUnderscore(subject.Name) {
		return fmt.Errorf(lang.PkgValidateErrPkgConstantName, subject.Name)
	}

	if !regexp.MustCompile(subject.Pattern).MatchString(subject.Value) {
		return fmt.Errorf(lang.PkgValidateErrPkgConstantPattern, subject.Name, subject.Pattern)
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
