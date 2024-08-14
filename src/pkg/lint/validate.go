// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation"
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

const (
	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength   = 40
	errChartReleaseNameEmpty = "release name empty, unable to fallback to chart name"
)

// Package errors found during validation.
const (
	PkgValidateErrInitNoYOLO              = "sorry, you can't YOLO an init package"
	PkgValidateErrConstant                = "invalid package constant: %w"
	PkgValidateErrYOLONoOCI               = "OCI images not allowed in YOLO"
	PkgValidateErrYOLONoGit               = "git repos not allowed in YOLO"
	PkgValidateErrYOLONoArch              = "cluster architecture not allowed in YOLO"
	PkgValidateErrYOLONoDistro            = "cluster distros not allowed in YOLO"
	PkgValidateErrComponentNameNotUnique  = "component name %q is not unique"
	PkgValidateErrComponentReqDefault     = "component %q cannot be both required and default"
	PkgValidateErrComponentReqGrouped     = "component %q cannot be both required and grouped"
	PkgValidateErrChartNameNotUnique      = "chart name %q is not unique"
	PkgValidateErrChart                   = "invalid chart definition: %w"
	PkgValidateErrManifestNameNotUnique   = "manifest name %q is not unique"
	PkgValidateErrManifest                = "invalid manifest definition: %w"
	PkgValidateErrGroupMultipleDefaults   = "group %q has multiple defaults (%q, %q)"
	PkgValidateErrGroupOneComponent       = "group %q only has one component (%q)"
	PkgValidateErrAction                  = "invalid action: %w"
	PkgValidateErrActionCmdWait           = "action %q cannot be both a command and wait action"
	PkgValidateErrActionClusterNetwork    = "a single wait action must contain only one of cluster or network"
	PkgValidateErrChartName               = "chart %q exceed the maximum length of %d characters"
	PkgValidateErrChartNamespaceMissing   = "chart %q must include a namespace"
	PkgValidateErrChartURLOrPath          = "chart %q must have either a url or localPath"
	PkgValidateErrChartVersion            = "chart %q must include a chart version"
	PkgValidateErrManifestFileOrKustomize = "manifest %q must have at least one file or kustomization"
	PkgValidateErrManifestNameLength      = "manifest %q exceed the maximum length of %d characters"
	PkgValidateErrVariable                = "invalid package variable: %w"
)

// ValidatePackage runs all validation checks on the package.
func ValidatePackage(pkg v1alpha1.ZarfPackage) error {
	var err error
	if pkg.Kind == v1alpha1.ZarfInitConfig && pkg.Metadata.YOLO {
		err = errors.Join(err, errors.New(PkgValidateErrInitNoYOLO))
	}
	for _, constant := range pkg.Constants {
		if varErr := constant.Validate(); varErr != nil {
			err = errors.Join(err, fmt.Errorf(PkgValidateErrConstant, varErr))
		}
	}
	uniqueComponentNames := make(map[string]bool)
	groupDefault := make(map[string]string)
	groupedComponents := make(map[string][]string)
	if pkg.Metadata.YOLO {
		for _, component := range pkg.Components {
			if len(component.Images) > 0 {
				err = errors.Join(err, errors.New(PkgValidateErrYOLONoOCI))
			}
			if len(component.Repos) > 0 {
				err = errors.Join(err, errors.New(PkgValidateErrYOLONoGit))
			}
			if component.Only.Cluster.Architecture != "" {
				err = errors.Join(err, errors.New(PkgValidateErrYOLONoArch))
			}
			if len(component.Only.Cluster.Distros) > 0 {
				err = errors.Join(err, errors.New(PkgValidateErrYOLONoDistro))
			}
		}
	}
	for _, component := range pkg.Components {
		// ensure component name is unique
		if _, ok := uniqueComponentNames[component.Name]; ok {
			err = errors.Join(err, fmt.Errorf(PkgValidateErrComponentNameNotUnique, component.Name))
		}
		uniqueComponentNames[component.Name] = true
		if component.IsRequired() {
			if component.Default {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrComponentReqDefault, component.Name))
			}
			if component.DeprecatedGroup != "" {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrComponentReqGrouped, component.Name))
			}
		}
		uniqueChartNames := make(map[string]bool)
		for _, chart := range component.Charts {
			// ensure chart name is unique
			if _, ok := uniqueChartNames[chart.Name]; ok {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrChartNameNotUnique, chart.Name))
			}
			uniqueChartNames[chart.Name] = true
			if chartErr := validateChart(chart); chartErr != nil {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrChart, chartErr))
			}
		}
		uniqueManifestNames := make(map[string]bool)
		for _, manifest := range component.Manifests {
			// ensure manifest name is unique
			if _, ok := uniqueManifestNames[manifest.Name]; ok {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrManifestNameNotUnique, manifest.Name))
			}
			uniqueManifestNames[manifest.Name] = true
			if manifestErr := validateManifest(manifest); manifestErr != nil {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrManifest, manifestErr))
			}
		}
		if actionsErr := validateActions(component.Actions); actionsErr != nil {
			err = errors.Join(err, fmt.Errorf("%q: %w", component.Name, actionsErr))
		}
		// ensure groups don't have multiple defaults or only one component
		if component.DeprecatedGroup != "" {
			if component.Default {
				if _, ok := groupDefault[component.DeprecatedGroup]; ok {
					err = errors.Join(err, fmt.Errorf(PkgValidateErrGroupMultipleDefaults, component.DeprecatedGroup, groupDefault[component.DeprecatedGroup], component.Name))
				}
				groupDefault[component.DeprecatedGroup] = component.Name
			}
			groupedComponents[component.DeprecatedGroup] = append(groupedComponents[component.DeprecatedGroup], component.Name)
		}
	}
	for groupKey, componentNames := range groupedComponents {
		if len(componentNames) == 1 {
			err = errors.Join(err, fmt.Errorf(PkgValidateErrGroupOneComponent, groupKey, componentNames[0]))
		}
	}
	return err
}

// validateActions validates the actions of a component.
func validateActions(a v1alpha1.ZarfComponentActions) error {
	var err error

	err = errors.Join(err, validateActionSet(a.OnCreate))

	if hasSetVariables(a.OnCreate) {
		err = errors.Join(err, fmt.Errorf("cannot contain setVariables outside of onDeploy in actions"))
	}

	err = errors.Join(err, validateActionSet(a.OnDeploy))

	if hasSetVariables(a.OnRemove) {
		err = errors.Join(err, fmt.Errorf("cannot contain setVariables outside of onDeploy in actions"))
	}

	err = errors.Join(err, validateActionSet(a.OnRemove))

	return err
}

// hasSetVariables returns true if any of the actions contain setVariables.
func hasSetVariables(as v1alpha1.ZarfComponentActionSet) bool {
	check := func(actions []v1alpha1.ZarfComponentAction) bool {
		for _, action := range actions {
			if len(action.SetVariables) > 0 {
				return true
			}
		}
		return false
	}

	return check(as.Before) || check(as.After) || check(as.OnSuccess) || check(as.OnFailure)
}

// validateActionSet runs all validation checks on component action sets.
func validateActionSet(as v1alpha1.ZarfComponentActionSet) error {
	var err error
	validate := func(actions []v1alpha1.ZarfComponentAction) {
		for _, action := range actions {
			if actionErr := validateAction(action); actionErr != nil {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrAction, actionErr))
			}
		}
	}

	validate(as.Before)
	validate(as.After)
	validate(as.OnFailure)
	validate(as.OnSuccess)
	return err
}

// validateAction runs all validation checks on an action.
func validateAction(action v1alpha1.ZarfComponentAction) error {
	var err error

	if action.Wait != nil {
		// Validate only cmd or wait, not both
		if action.Cmd != "" {
			err = errors.Join(err, fmt.Errorf(PkgValidateErrActionCmdWait, action.Cmd))
		}

		// Validate only cluster or network, not both
		if action.Wait.Cluster != nil && action.Wait.Network != nil {
			err = errors.Join(err, errors.New(PkgValidateErrActionClusterNetwork))
		}

		// Validate at least one of cluster or network
		if action.Wait.Cluster == nil && action.Wait.Network == nil {
			err = errors.Join(err, errors.New(PkgValidateErrActionClusterNetwork))
		}
	}

	return err
}

// validateReleaseName validates a release name against DNS 1035 spec, using chartName as fallback.
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#rfc-1035-label-names
func validateReleaseName(chartName, releaseName string) (err error) {
	// Fallback to chartName if releaseName is empty
	// NOTE: Similar fallback mechanism happens in src/internal/packager/helm/chart.go:InstallOrUpgradeChart
	if releaseName == "" {
		releaseName = chartName
	}

	// Check if the final releaseName is empty and return an error if so
	if releaseName == "" {
		err = errors.New(errChartReleaseNameEmpty)
		return
	}

	// Validate the releaseName against DNS 1035 label spec
	if errs := validation.IsDNS1035Label(releaseName); len(errs) > 0 {
		err = fmt.Errorf("invalid release name '%s': %s", releaseName, strings.Join(errs, "; "))
	}

	return
}

// validateChart runs all validation checks on a chart.
func validateChart(chart v1alpha1.ZarfChart) error {
	var err error

	if len(chart.Name) > ZarfMaxChartNameLength {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartName, chart.Name, ZarfMaxChartNameLength))
	}

	if chart.Namespace == "" {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartNamespaceMissing, chart.Name))
	}

	// Must have a url or localPath (and not both)
	if chart.URL != "" && chart.LocalPath != "" {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartURLOrPath, chart.Name))
	}

	if chart.URL == "" && chart.LocalPath == "" {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartURLOrPath, chart.Name))
	}

	if chart.Version == "" {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartVersion, chart.Name))
	}

	if nameErr := validateReleaseName(chart.Name, chart.ReleaseName); nameErr != nil {
		err = errors.Join(err, nameErr)
	}

	return err
}

// validateManifest runs all validation checks on a manifest.
func validateManifest(manifest v1alpha1.ZarfManifest) error {
	var err error

	if len(manifest.Name) > ZarfMaxChartNameLength {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrManifestNameLength, manifest.Name, ZarfMaxChartNameLength))
	}

	if len(manifest.Files) < 1 && len(manifest.Kustomizations) < 1 {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrManifestFileOrKustomize, manifest.Name))
	}

	return err
}
