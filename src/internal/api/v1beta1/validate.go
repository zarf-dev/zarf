// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"errors"
	"fmt"
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
	PkgValidateErrActionSetValueOnDeploy  = "cannot contain setValues outside of onDeploy in actions"
	PkgValidateErrActionTemplateOnCreate  = "templating is not supported in onCreate actions"
	PkgValidateErrChartName               = "chart %q exceed the maximum length of %d characters"
	PkgValidateErrChartNamespaceMissing   = "chart %q must include a namespace"
	PkgValidateErrChartSource             = "chart %q must have exactly one source (helmRepository, git, local, or oci)"
	PkgValidateErrManifestFileOrKustomize = "manifest %q must have at least one file or kustomization"
	PkgValidateErrManifestNameLength      = "manifest %q exceed the maximum length of %d characters"
	PkgValidateErrNoComponents            = "package does not contain any compatible components"
)

// ValidatePackage runs all validation checks on the package.
func ValidatePackage(pkg v1beta1.Package) error {
	var err error
	if len(pkg.Components) == 0 {
		err = errors.Join(err, errors.New(PkgValidateErrNoComponents))
	}
	uniqueComponentNames := make(map[string]bool)
	for _, component := range pkg.Components {
		// ensure component name is unique
		if _, ok := uniqueComponentNames[component.Name]; ok {
			err = errors.Join(err, fmt.Errorf(PkgValidateErrComponentNameNotUnique, component.Name))
		}
		uniqueComponentNames[component.Name] = true

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
	}

	return err
}

// validateActions validates the actions of a component.
func validateActions(a v1beta1.ComponentActions) error {
	var err error

	err = errors.Join(err, validateActionSet(a.OnCreate))

	if hasSetValues(a.OnCreate) {
		err = errors.Join(err, errors.New(PkgValidateErrActionSetValueOnDeploy))
	}

	if hasTemplating(a.OnCreate) {
		err = errors.Join(err, errors.New(PkgValidateErrActionTemplateOnCreate))
	}

	err = errors.Join(err, validateActionSet(a.OnDeploy))
	err = errors.Join(err, validateActionSet(a.OnRemove))

	return err
}

// hasSetValues returns true if any of the actions contain setValues.
func hasSetValues(as v1beta1.ComponentActionSet) bool {
	check := func(actions []v1beta1.ComponentAction) bool {
		for _, action := range actions {
			if len(action.SetValues) > 0 {
				return true
			}
		}
		return false
	}

	return check(as.Before) || check(as.OnSuccess) || check(as.OnFailure)
}

// hasTemplating returns true if any of the actions have templating enabled.
func hasTemplating(as v1beta1.ComponentActionSet) bool {
	check := func(actions []v1beta1.ComponentAction) bool {
		for _, action := range actions {
			if action.EnableTemplating {
				return true
			}
		}
		return false
	}

	return check(as.Before) || check(as.OnSuccess) || check(as.OnFailure)
}

// validateActionSet runs all validation checks on component action sets.
func validateActionSet(as v1beta1.ComponentActionSet) error {
	var err error
	validate := func(actions []v1beta1.ComponentAction) {
		for _, action := range actions {
			if actionErr := validateAction(action); actionErr != nil {
				err = errors.Join(err, fmt.Errorf(PkgValidateErrAction, actionErr))
			}
		}
	}

	validate(as.Before)
	validate(as.OnFailure)
	validate(as.OnSuccess)
	return err
}

// validateAction runs all validation checks on an action.
func validateAction(action v1beta1.ComponentAction) error {
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
func validateChart(chart v1beta1.Chart) error {
	var err error

	if len(chart.Name) > ZarfMaxChartNameLength {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartName, chart.Name, ZarfMaxChartNameLength))
	}

	if chart.Namespace == "" {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartNamespaceMissing, chart.Name))
	}

	// Must have exactly one source
	sources := 0
	for _, set := range []bool{chart.HelmRepository != nil, chart.Git != nil, chart.Local != nil, chart.OCI != nil} {
		if set {
			sources++
		}
	}
	if sources != 1 {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrChartSource, chart.Name))
	}

	if nameErr := validateReleaseName(chart.Name, chart.ReleaseName); nameErr != nil {
		err = errors.Join(err, nameErr)
	}

	return err
}

// validateManifest runs all validation checks on a manifest.
func validateManifest(manifest v1beta1.Manifest) error {
	var err error

	if len(manifest.Name) > ZarfMaxChartNameLength {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrManifestNameLength, manifest.Name, ZarfMaxChartNameLength))
	}

	if len(manifest.Files) < 1 && manifest.Kustomize == nil {
		err = errors.Join(err, fmt.Errorf(PkgValidateErrManifestFileOrKustomize, manifest.Name))
	}

	return err
}
