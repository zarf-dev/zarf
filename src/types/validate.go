// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config/lang"
)

const (
	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength = 40
)

var (
	// IsLowercaseNumberHyphenNoStartHyphen is a regex for lowercase, numbers and hyphens that cannot start with a hyphen.
	// https://regex101.com/r/FLdG9G/2
	IsLowercaseNumberHyphenNoStartHyphen = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]*$`).MatchString
	// Define allowed OS, an empty string means it is allowed on all operating systems
	// same as enums on ZarfComponentOnlyTarget
	supportedOS = []string{"linux", "darwin", "windows", ""}
)

// SupportedOS returns the supported operating systems.
//
// The supported operating systems are: linux, darwin, windows.
//
// An empty string signifies no OS restrictions.
func SupportedOS() []string {
	return supportedOS
}

// Validate runs all validation checks on the package.
func (pkg ZarfPackage) Validate() error {
	errs := []error{}
	if pkg.Kind == ZarfInitConfig && pkg.Metadata.YOLO {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrInitNoYOLO))
	}

	if !IsLowercaseNumberHyphenNoStartHyphen(pkg.Metadata.Name) {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrPkgName, pkg.Metadata.Name))
	}

	if len(pkg.Components) == 0 {
		errs = append(errs, fmt.Errorf("package must have at least 1 component"))
	}

	for _, variable := range pkg.Variables {
		if err := variable.Validate(); err != nil {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrVariable, err))
		}
	}

	for _, constant := range pkg.Constants {
		if err := constant.Validate(); err != nil {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrConstant, err))
		}
	}

	uniqueComponentNames := make(map[string]bool)
	groupDefault := make(map[string]string)
	groupedComponents := make(map[string][]string)

	if pkg.Metadata.YOLO {
		for _, component := range pkg.Components {
			if len(component.Images) > 0 {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrYOLONoOCI))
			}

			if len(component.Repos) > 0 {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrYOLONoGit))
			}

			if component.Only.Cluster.Architecture != "" {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrYOLONoArch))
			}

			if len(component.Only.Cluster.Distros) > 0 {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrYOLONoDistro))
			}
		}
	}

	for _, component := range pkg.Components {
		// ensure component name is unique
		if _, ok := uniqueComponentNames[component.Name]; ok {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrComponentNameNotUnique, component.Name))
		}
		uniqueComponentNames[component.Name] = true

		if !IsLowercaseNumberHyphenNoStartHyphen(component.Name) {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrComponentName, component.Name))
		}

		if !slices.Contains(supportedOS, component.Only.LocalOS) {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrComponentLocalOS, component.Name, component.Only.LocalOS, supportedOS))
		}

		if component.IsRequired() {
			if component.Default {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrComponentReqDefault, component.Name))
			}
			if component.DeprecatedGroup != "" {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrComponentReqGrouped, component.Name))
			}
		}

		uniqueChartNames := make(map[string]bool)
		for _, chart := range component.Charts {
			// ensure chart name is unique
			if _, ok := uniqueChartNames[chart.Name]; ok {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartNameNotUnique, chart.Name))
			}
			uniqueChartNames[chart.Name] = true

			if err := chart.Validate(); err != nil {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrChart, err))
			}
		}

		uniqueManifestNames := make(map[string]bool)
		for _, manifest := range component.Manifests {
			// ensure manifest name is unique
			if _, ok := uniqueManifestNames[manifest.Name]; ok {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrManifestNameNotUnique, manifest.Name))
			}
			uniqueManifestNames[manifest.Name] = true

			if err := manifest.Validate(); err != nil {
				errs = append(errs, fmt.Errorf(lang.PkgValidateErrManifest, err))
			}
		}

		if err := component.Actions.validate(); err != nil {
			errs = append(errs, fmt.Errorf("%q: %w", component.Name, err))
		}

		// ensure groups don't have multiple defaults or only one component
		if component.DeprecatedGroup != "" {
			if component.Default {
				if _, ok := groupDefault[component.DeprecatedGroup]; ok {
					errs = append(errs, fmt.Errorf(lang.PkgValidateErrGroupMultipleDefaults, component.DeprecatedGroup, groupDefault[component.DeprecatedGroup], component.Name))
				}
				groupDefault[component.DeprecatedGroup] = component.Name
			}
			groupedComponents[component.DeprecatedGroup] = append(groupedComponents[component.DeprecatedGroup], component.Name)
		}
	}

	for groupKey, componentNames := range groupedComponents {
		if len(componentNames) == 1 {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrGroupOneComponent, groupKey, componentNames[0]))
		}
	}

	return errors.Join(errs...)
}

func (a ZarfComponentActions) validate() error {
	var errs []error

	if err := a.OnCreate.Validate(); err != nil {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrAction, err))
	}

	if a.OnCreate.HasSetVariables() {
		errs = append(errs, fmt.Errorf("cannot contain setVariables outside of onDeploy in actions"))
	}

	if err := a.OnDeploy.Validate(); err != nil {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrAction, err))
	}

	if a.OnRemove.HasSetVariables() {
		errs = append(errs, fmt.Errorf("cannot contain setVariables outside of onDeploy in actions"))
	}

	if err := a.OnRemove.Validate(); err != nil {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrAction, err))
	}

	return errors.Join(errs...)
}

// Validate validates the component trying to be imported.
func (c ZarfComponent) Validate() error {
	errs := []error{}
	path := c.Import.Path
	url := c.Import.URL

	// ensure path or url is provided
	if path == "" && url == "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrImportDefinition, c.Name, "neither a path nor a URL was provided"))
	}

	// ensure path and url are not both provided
	if path != "" && url != "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrImportDefinition, c.Name, "both a path and a URL were provided"))
	}

	// validation for path
	if url == "" && path != "" {
		// ensure path is not an absolute path
		if filepath.IsAbs(path) {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrImportDefinition, c.Name, "path cannot be an absolute path"))
		}
	}

	// validation for url
	if url != "" && path == "" {
		ok := helpers.IsOCIURL(url)
		if !ok {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrImportDefinition, c.Name, "URL is not a valid OCI URL"))
		}
	}

	return errors.Join(errs...)
}

// HasSetVariables returns true if any of the actions contain setVariables.
func (as ZarfComponentActionSet) HasSetVariables() bool {
	check := func(actions []ZarfComponentAction) bool {
		for _, action := range actions {
			if len(action.SetVariables) > 0 {
				return true
			}
		}
		return false
	}

	return check(as.Before) || check(as.After) || check(as.OnSuccess) || check(as.OnFailure)
}

// Validate runs all validation checks on component action sets.
func (as ZarfComponentActionSet) Validate() error {
	validate := func(actions []ZarfComponentAction) error {
		for _, action := range actions {
			if err := action.Validate(); err != nil {
				return err
			}
		}
		return nil
	}

	if err := validate(as.Before); err != nil {
		return err
	}
	if err := validate(as.After); err != nil {
		return err
	}
	if err := validate(as.OnSuccess); err != nil {
		return err
	}
	return validate(as.OnFailure)
}

// Validate runs all validation checks on an action.
func (action ZarfComponentAction) Validate() error {
	errs := []error{}
	// Validate SetVariable
	for _, variable := range action.SetVariables {
		if err := variable.Validate(); err != nil {
			errs = append(errs, err)
		}
	}

	if action.Wait != nil {
		// Validate only cmd or wait, not both
		if action.Cmd != "" {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrActionCmdWait, action.Cmd))
		}

		// Validate only cluster or network, not both
		if action.Wait.Cluster != nil && action.Wait.Network != nil {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrActionClusterNetwork))
		}

		// Validate at least one of cluster or network
		if action.Wait.Cluster == nil && action.Wait.Network == nil {
			errs = append(errs, fmt.Errorf(lang.PkgValidateErrActionClusterNetwork))
		}
	}

	return errors.Join(errs...)
}

// Validate runs all validation checks on a chart.
func (chart ZarfChart) Validate() error {
	errs := []error{}

	if chart.Name == "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartNameMissing))
	}

	if len(chart.Name) > ZarfMaxChartNameLength {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartName, chart.Name, ZarfMaxChartNameLength))
	}

	if chart.Namespace == "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartNamespaceMissing, chart.Name))
	}

	// Must have a url or localPath (and not both)
	if chart.URL != "" && chart.LocalPath != "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartURLOrPath, chart.Name))
	}

	if chart.URL == "" && chart.LocalPath == "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartURLOrPath, chart.Name))
	}

	if chart.Version == "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrChartVersion, chart.Name))
	}

	return errors.Join(errs...)
}

// Validate runs all validation checks on a manifest.
func (manifest ZarfManifest) Validate() error {
	errs := []error{}

	if manifest.Name == "" {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrManifestNameMissing))
	}

	if len(manifest.Name) > ZarfMaxChartNameLength {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrManifestNameLength, manifest.Name, ZarfMaxChartNameLength))
	}

	if len(manifest.Files) < 1 && len(manifest.Kustomizations) < 1 {
		errs = append(errs, fmt.Errorf(lang.PkgValidateErrManifestFileOrKustomize, manifest.Name))
	}

	return errors.Join(errs...)
}
