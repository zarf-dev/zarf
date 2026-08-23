// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// ZarfMaxChartNameLength limits helm chart name size to account for K8s/helm limits and zarf prefix
	ZarfMaxChartNameLength   = 40
	errChartReleaseNameEmpty = "release name empty, unable to fallback to chart name"
)

// Package errors found during validation.
const (
	PkgValidateErrComponentNameNotUnique  = "component name %q is not unique"
	PkgValidateErrChartNameNotUnique      = "chart name %q is not unique"
	PkgValidateErrChart                   = "invalid chart definition: %w"
	PkgValidateErrManifestNameNotUnique   = "manifest name %q is not unique"
	PkgValidateErrManifest                = "invalid manifest definition: %w"
	PkgValidateErrAction                  = "invalid action: %w"
	PkgValidateErrActionCmdWait           = "action %q cannot be both a command and wait action"
	PkgValidateErrActionClusterNetwork    = "a single wait action must contain only one of cluster or network"
	PkgValidateErrActionSetValueOnDeploy  = "setValues is not supported in onCreate actions"
	PkgValidateErrActionTemplateOnCreate  = "templating is not supported in onCreate actions"
	PkgValidateErrChartName               = "chart %q exceed the maximum length of %d characters"
	PkgValidateErrChartNamespaceMissing   = "chart %q must include a namespace"
	PkgValidateErrManifestFileOrKustomize = "manifest %q must have at least one file or kustomization"
	PkgValidateErrManifestNameLength      = "manifest %q exceed the maximum length of %d characters"
	PkgValidateErrNoComponents            = "package does not contain any compatible components"
)

// ValidationErrors contains all errors found during package validation.
type ValidationErrors []error

// Error returns the validation errors separated by newlines.
func (errs ValidationErrors) Error() string {
	var builder strings.Builder
	for _, err := range errs {
		if err == nil {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(err.Error())
	}
	return builder.String()
}

// Unwrap returns the individual validation errors.
func (errs ValidationErrors) Unwrap() []error {
	return errs
}

// ValidatePackage runs all validation checks on the package.
func ValidatePackage(pkg v1beta1.Package) ValidationErrors {
	var errs ValidationErrors
	if len(pkg.Components) == 0 {
		errs = append(errs, errors.New(PkgValidateErrNoComponents))
	}
	uniqueComponentNames := make(map[string]bool)
	for _, component := range pkg.Components {
		// ensure component name is unique
		if _, ok := uniqueComponentNames[component.Name]; ok {
			errs = append(errs, fmt.Errorf(PkgValidateErrComponentNameNotUnique, component.Name))
		}
		uniqueComponentNames[component.Name] = true

		uniqueChartNames := make(map[string]bool)
		for _, chart := range component.Charts {
			// ensure chart name is unique
			if _, ok := uniqueChartNames[chart.Name]; ok {
				errs = append(errs, fmt.Errorf(PkgValidateErrChartNameNotUnique, chart.Name))
			}
			uniqueChartNames[chart.Name] = true
			for _, chartErr := range validateChart(chart) {
				errs = append(errs, fmt.Errorf(PkgValidateErrChart, chartErr))
			}
		}
		uniqueManifestNames := make(map[string]bool)
		for _, manifest := range component.Manifests {
			// ensure manifest name is unique
			if _, ok := uniqueManifestNames[manifest.Name]; ok {
				errs = append(errs, fmt.Errorf(PkgValidateErrManifestNameNotUnique, manifest.Name))
			}
			uniqueManifestNames[manifest.Name] = true
			for _, manifestErr := range validateManifest(manifest) {
				errs = append(errs, fmt.Errorf(PkgValidateErrManifest, manifestErr))
			}
		}
		for _, actionsErr := range validateActions(component.Actions) {
			errs = append(errs, fmt.Errorf("%q: %w", component.Name, actionsErr))
		}
	}

	return errs
}

// validateActions validates the actions of a component.
func validateActions(a v1beta1.ComponentActions) ValidationErrors {
	var errs ValidationErrors

	errs = append(errs, validateActionSet(a.OnCreate)...)

	if hasSetValues(a.OnCreate) {
		errs = append(errs, errors.New(PkgValidateErrActionSetValueOnDeploy))
	}

	if hasTemplating(a.OnCreate) {
		errs = append(errs, errors.New(PkgValidateErrActionTemplateOnCreate))
	}

	errs = append(errs, validateActionSet(a.OnDeploy)...)
	errs = append(errs, validateActionSet(a.OnRemove)...)

	return errs
}

// hasSetValues returns true if any of the actions contain setValues.
func hasSetValues(as v1beta1.ComponentActionSet) bool {
	return slices.ContainsFunc(as.Before, hasActionSetValues) ||
		slices.ContainsFunc(as.OnSuccess, hasActionSetValues) ||
		slices.ContainsFunc(as.OnFailure, hasActionSetValues)
}

// hasTemplating returns true if any of the actions have templating enabled.
func hasTemplating(as v1beta1.ComponentActionSet) bool {
	return slices.ContainsFunc(as.Before, hasActionTemplating) ||
		slices.ContainsFunc(as.OnSuccess, hasActionTemplating) ||
		slices.ContainsFunc(as.OnFailure, hasActionTemplating)
}

func hasActionSetValues(action v1beta1.ComponentAction) bool {
	return len(action.SetValues) > 0
}

func hasActionTemplating(action v1beta1.ComponentAction) bool {
	return action.EnableTemplating
}

// validateActionSet runs all validation checks on component action sets.
func validateActionSet(as v1beta1.ComponentActionSet) ValidationErrors {
	var errs ValidationErrors
	validate := func(actions []v1beta1.ComponentAction) {
		for _, action := range actions {
			for _, actionErr := range validateAction(action) {
				errs = append(errs, fmt.Errorf(PkgValidateErrAction, actionErr))
			}
		}
	}

	validate(as.Before)
	validate(as.OnFailure)
	validate(as.OnSuccess)
	return errs
}

// validateAction runs all validation checks on an action.
func validateAction(action v1beta1.ComponentAction) ValidationErrors {
	if action.Wait == nil {
		return nil
	}

	var errs ValidationErrors

	// Validate only cmd or wait, not both
	if action.Cmd != "" {
		errs = append(errs, fmt.Errorf(PkgValidateErrActionCmdWait, action.Cmd))
	}

	// Validate only cluster or network, not both
	if action.Wait.Cluster != nil && action.Wait.Network != nil {
		errs = append(errs, errors.New(PkgValidateErrActionClusterNetwork))
	}

	// Validate at least one of cluster or network
	if action.Wait.Cluster == nil && action.Wait.Network == nil {
		errs = append(errs, errors.New(PkgValidateErrActionClusterNetwork))
	}

	return errs
}

// validateReleaseName validates a release name against DNS 1035 spec, using chartName as fallback.
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#rfc-1035-label-names
func validateReleaseName(chartName, releaseName string) error {
	// Fallback to chartName if releaseName is empty
	// NOTE: Similar fallback mechanism happens in src/internal/packager/helm/chart.go:InstallOrUpgradeChart
	if releaseName == "" {
		releaseName = chartName
	}

	// Check if the final releaseName is empty and return an error if so
	if releaseName == "" {
		return errors.New(errChartReleaseNameEmpty)
	}

	// Validate the releaseName against DNS 1035 label spec
	if errs := validation.IsDNS1035Label(releaseName); len(errs) > 0 {
		return fmt.Errorf("invalid release name '%s': %s", releaseName, strings.Join(errs, "; "))
	}

	return nil
}

// validateChart runs all validation checks on a chart.
func validateChart(chart v1beta1.Chart) ValidationErrors {
	var errs ValidationErrors

	if len(chart.Name) > ZarfMaxChartNameLength {
		errs = append(errs, fmt.Errorf(PkgValidateErrChartName, chart.Name, ZarfMaxChartNameLength))
	}

	if chart.Namespace == "" {
		errs = append(errs, fmt.Errorf(PkgValidateErrChartNamespaceMissing, chart.Name))
	}

	if nameErr := validateReleaseName(chart.Name, chart.ReleaseName); nameErr != nil {
		errs = append(errs, nameErr)
	}

	return errs
}

// validateManifest runs all validation checks on a manifest.
func validateManifest(manifest v1beta1.Manifest) ValidationErrors {
	var errs ValidationErrors

	if len(manifest.Name) > ZarfMaxChartNameLength {
		errs = append(errs, fmt.Errorf(PkgValidateErrManifestNameLength, manifest.Name, ZarfMaxChartNameLength))
	}

	if len(manifest.Files) < 1 && manifest.Kustomize == nil {
		errs = append(errs, fmt.Errorf(PkgValidateErrManifestFileOrKustomize, manifest.Name))
	}

	return errs
}
