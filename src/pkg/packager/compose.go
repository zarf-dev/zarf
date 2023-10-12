// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// composeComponents builds the composed components list for the current config.
func (p *Packager) composeComponents() error {
	components := []types.ZarfComponent{}

	pkgVars := p.cfg.Pkg.Variables
	pkgConsts := p.cfg.Pkg.Constants

	for _, component := range p.cfg.Pkg.Components {
		// build the import chain
		start := time.Now()
		chain, err := composer.NewImportChain(component, p.arch)
		if err != nil {
			return err
		}
		message.Debugf("Building import chain for %q took %s", component.Name, time.Since(start))

		// migrate any deprecated component configurations now
		start = time.Now()
		warnings := chain.Migrate(p.cfg.Pkg.Build)
		p.warnings = append(p.warnings, warnings...)
		message.Debugf("Migrating %q took %s", component.Name, time.Since(start))

		// get the composed component
		start = time.Now()
		composed := chain.Compose()
		components = append(components, composed)
		message.Debugf("Composing %q took %s", component.Name, time.Since(start))

		// merge variables and constants
		start = time.Now()
		pkgVars = chain.MergeVariables(pkgVars)
		pkgConsts = chain.MergeConstants(pkgConsts)
		message.Debugf("Merging pkg vars and consts from import chain took %s", time.Since(start))
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	p.cfg.Pkg.Components = components

	return nil
}

// getComposedComponent recursively retrieves a composed Zarf component
// --------------------------------------------------------------------
// For composed components, we build the tree of components starting at the root and adding children as we go;
// this follows the composite design pattern outlined here: https://en.wikipedia.org/wiki/Composite_pattern
// where 1 component parent is made up of 0...n composite or leaf children.
func (p *Packager) getComposedComponent(parentComponent types.ZarfComponent) (child types.ZarfComponent, err error) {
	// Make sure the component we're trying to import can't be accessed.
	if err := validate.ImportDefinition(&parentComponent); err != nil {
		return child, fmt.Errorf("invalid import definition in the %s component: %w", parentComponent.Name, err)
	}

	// Keep track of the composed components import path to build nested composed components.
	pathAncestry := ""

	// Get the component that we are trying to import.
	// NOTE: This function is recursive and will continue getting the children until there are no more 'imported' components left.
	child, err = p.getChildComponent(parentComponent, pathAncestry)
	if err != nil {
		return child, fmt.Errorf("unable to get child component: %w", err)
	}

	// Merge the overrides from the child that we just received with the parent we were provided.
	p.mergeComponentOverrides(&child, parentComponent)

	return
}

func (p *Packager) getChildComponent(parent types.ZarfComponent, pathAncestry string) (child types.ZarfComponent, err error) {
	// Figure out which component we are actually importing.
	// NOTE: Default to the component name if a custom one was not provided.
	childComponentName := parent.Import.ComponentName
	if childComponentName == "" {
		childComponentName = parent.Name
	}

	var cachePath string

	subPkgPaths := layout.New(parent.Import.Path)

	if parent.Import.URL != "" {
		if !strings.HasSuffix(parent.Import.URL, oci.SkeletonSuffix) {
			return child, fmt.Errorf("import URL must be a 'skeleton' package: %s", parent.Import.URL)
		}

		// Save all the OCI imported components into our build data
		p.cfg.Pkg.Build.OCIImportedComponents[parent.Import.URL] = childComponentName

		skelURL := strings.TrimPrefix(parent.Import.URL, helpers.OCIURLPrefix)
		cachePath = filepath.Join(config.GetAbsCachePath(), "oci", skelURL)
		err = os.MkdirAll(cachePath, 0755)
		if err != nil {
			return child, fmt.Errorf("unable to create cache path %s: %w", cachePath, err)
		}

		err = p.setOCIRemote(parent.Import.URL)
		if err != nil {
			return child, err
		}
		manifest, err := p.remote.FetchRoot()
		if err != nil {
			return child, err
		}
		tb := filepath.Join(layout.ComponentsDir, fmt.Sprintf("%s.tar", childComponentName))
		fetchedLayers, err := p.remote.PullPackage(cachePath, config.CommonOptions.OCIConcurrency, manifest.Locate(tb))
		if err != nil {
			return child, fmt.Errorf("unable to pull skeleton from %s: %w", skelURL, err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return child, fmt.Errorf("unable to get current working directory: %w", err)
		}

		rel, err := filepath.Rel(cwd, cachePath)
		if err != nil {
			return child, fmt.Errorf("unable to get relative path: %w", err)
		}
		parent.Import.Path = rel
		subPkgPaths = layout.New(parent.Import.Path)
		subPkgPaths.SetFromLayers(fetchedLayers)
	}

	var subPkg types.ZarfPackage
	if err := utils.ReadYaml(filepath.Join(pathAncestry, subPkgPaths.ZarfYAML), &subPkg); err != nil {
		return child, err
	}

	// Merge in child package variables (only if the variable does not exist in parent).
	for _, importedVariable := range subPkg.Variables {
		p.injectImportedVariable(importedVariable)
	}

	// Merge in child package constants (only if the constant does not exist in parent).
	for _, importedConstant := range subPkg.Constants {
		p.injectImportedConstant(importedConstant)
	}

	// Find the child component from the imported package that matches our arch.
	for _, component := range subPkg.Components {
		if component.Name == childComponentName {
			filterArch := component.Only.Cluster.Architecture

			// Override the filter if it is set by the parent component.
			if parent.Only.Cluster.Architecture != "" {
				filterArch = parent.Only.Cluster.Architecture
			}

			// Only add this component if it is valid for the target architecture.
			if filterArch == "" || filterArch == p.arch {
				child = component
				break
			}
		}
	}

	// If we didn't find a child component, bail.
	if child.Name == "" {
		return child, fmt.Errorf("unable to find the component %s in the imported package", childComponentName)
	}

	// If it's OCI, we need to unpack the component tarball
	if parent.Import.URL != "" {
		parent.Import.Path = filepath.Join(parent.Import.Path, layout.ComponentsDir, child.Name)
		if err := subPkgPaths.Components.Unarchive(child); err != nil {
			if layout.IsNotLoaded(err) {
				// If the tarball doesn't exist (skeleton component had no local resources), we need to create the directory anyways in case there are actions
				_, err := subPkgPaths.Components.Create(child)
				if err != nil {
					return child, fmt.Errorf("unable to create composed component cache path %s: %w", cachePath, err)
				}
			} else {
				return child, fmt.Errorf("unable to unarchive component: %w", err)
			}
		}
	}

	pathAncestry = filepath.Join(pathAncestry, parent.Import.Path)
	// Check if we need to get more of children.
	if child.Import.Path != "" {
		// Recursively call this function to get the next layer of children.
		grandchildComponent, err := p.getChildComponent(child, pathAncestry)
		if err != nil {
			return child, err
		}

		// Merge the grandchild values into the child.
		p.mergeComponentOverrides(&grandchildComponent, child)

		// Set the grandchild as the child component now that we're done with recursively importing.
		child = grandchildComponent
	} else {
		// Fix the filePaths of imported components to be accessible from our current location.
		child, err = p.fixComposedFilepaths(pathAncestry, child)
		if err != nil {
			return child, fmt.Errorf("unable to fix composed filepaths: %s", err.Error())
		}
	}

	// Migrate any deprecated component configurations now
	var warnings []string
	child, warnings = deprecated.MigrateComponent(p.cfg.Pkg.Build, child)
	p.warnings = append(p.warnings, warnings...)

	return
}

func (p *Packager) fixComposedFilepaths(pathAncestry string, child types.ZarfComponent) (types.ZarfComponent, error) {
	for fileIdx, file := range child.Files {
		composed := p.getComposedFilePath(pathAncestry, file.Source)
		child.Files[fileIdx].Source = composed
	}

	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			composed := p.getComposedFilePath(pathAncestry, valuesFile)
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = composed
		}
		if child.Charts[chartIdx].LocalPath != "" {
			composed := p.getComposedFilePath(pathAncestry, child.Charts[chartIdx].LocalPath)
			child.Charts[chartIdx].LocalPath = composed
		}
	}

	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			composed := p.getComposedFilePath(pathAncestry, file)
			child.Manifests[manifestIdx].Files[fileIdx] = composed
		}
		for kustomizeIdx, kustomization := range manifest.Kustomizations {
			composed := p.getComposedFilePath(pathAncestry, kustomization)
			// kustomizations can use non-standard urls, so we need to check if the composed path exists on the local filesystem
			abs, _ := filepath.Abs(composed)
			invalid := utils.InvalidPath(abs)
			if !invalid {
				child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = composed
			}
		}
	}

	for dataInjectionsIdx, dataInjection := range child.DataInjections {
		composed := p.getComposedFilePath(pathAncestry, dataInjection.Source)
		child.DataInjections[dataInjectionsIdx].Source = composed
	}

	var err error

	if child.Actions.OnCreate.OnSuccess, err = p.fixComposedActionFilepaths(pathAncestry, child.Actions.OnCreate.OnSuccess); err != nil {
		return child, err
	}
	if child.Actions.OnCreate.OnFailure, err = p.fixComposedActionFilepaths(pathAncestry, child.Actions.OnCreate.OnFailure); err != nil {
		return child, err
	}
	if child.Actions.OnCreate.Before, err = p.fixComposedActionFilepaths(pathAncestry, child.Actions.OnCreate.Before); err != nil {
		return child, err
	}
	if child.Actions.OnCreate.After, err = p.fixComposedActionFilepaths(pathAncestry, child.Actions.OnCreate.After); err != nil {
		return child, err
	}

	totalActions := len(child.Actions.OnCreate.OnSuccess) + len(child.Actions.OnCreate.OnFailure) + len(child.Actions.OnCreate.Before) + len(child.Actions.OnCreate.After)

	if totalActions > 0 {
		composedDefaultDir := p.getComposedFilePath(pathAncestry, child.Actions.OnCreate.Defaults.Dir)
		child.Actions.OnCreate.Defaults.Dir = composedDefaultDir
	}

	if child.DeprecatedCosignKeyPath != "" {
		composed := p.getComposedFilePath(pathAncestry, child.DeprecatedCosignKeyPath)
		child.DeprecatedCosignKeyPath = composed
	}

	child = p.composeExtensions(pathAncestry, child)

	return child, nil
}

func (p *Packager) fixComposedActionFilepaths(pathAncestry string, actions []types.ZarfComponentAction) ([]types.ZarfComponentAction, error) {
	for actionIdx, action := range actions {
		if action.Dir != nil {
			composedActionDir := p.getComposedFilePath(pathAncestry, *action.Dir)
			actions[actionIdx].Dir = &composedActionDir
		}
	}

	return actions, nil
}

// Sets Name, Default, Required and Description to the original components values.
func (p *Packager) mergeComponentOverrides(target *types.ZarfComponent, override types.ZarfComponent) {
	target.Name = override.Name
	target.Group = override.Group
	target.Default = override.Default
	target.Required = override.Required

	// Override description if it was provided.
	if override.Description != "" {
		target.Description = override.Description
	}

	// Override cosign key path if it was provided.
	if override.DeprecatedCosignKeyPath != "" {
		target.DeprecatedCosignKeyPath = override.DeprecatedCosignKeyPath
	}

	// Append slices where they exist.
	target.DataInjections = append(target.DataInjections, override.DataInjections...)
	target.Files = append(target.Files, override.Files...)
	target.Images = append(target.Images, override.Images...)
	target.Repos = append(target.Repos, override.Repos...)

	// Merge charts with the same name to keep them unique
	for _, overrideChart := range override.Charts {
		existing := false
		for idx := range target.Charts {
			if target.Charts[idx].Name == overrideChart.Name {
				if overrideChart.Namespace != "" {
					target.Charts[idx].Namespace = overrideChart.Namespace
				}
				if overrideChart.ReleaseName != "" {
					target.Charts[idx].ReleaseName = overrideChart.ReleaseName
				}
				target.Charts[idx].ValuesFiles = append(target.Charts[idx].ValuesFiles, overrideChart.ValuesFiles...)
				existing = true
			}
		}

		if !existing {
			target.Charts = append(target.Charts, overrideChart)
		}
	}

	// Merge manifests with the same name to keep them unique
	for _, overrideManifest := range override.Manifests {
		existing := false
		for idx := range target.Manifests {
			if target.Manifests[idx].Name == overrideManifest.Name {
				if overrideManifest.Namespace != "" {
					target.Manifests[idx].Namespace = overrideManifest.Namespace
				}
				target.Manifests[idx].Files = append(target.Manifests[idx].Files, overrideManifest.Files...)
				target.Manifests[idx].Kustomizations = append(target.Manifests[idx].Kustomizations, overrideManifest.Kustomizations...)

				existing = true
			}
		}

		if !existing {
			target.Manifests = append(target.Manifests, overrideManifest)
		}
	}

	// Check for nil array
	if override.Extensions.BigBang != nil {
		if override.Extensions.BigBang.ValuesFiles != nil {
			target.Extensions.BigBang.ValuesFiles = append(target.Extensions.BigBang.ValuesFiles, override.Extensions.BigBang.ValuesFiles...)
		}
		if override.Extensions.BigBang.FluxPatchFiles != nil {
			target.Extensions.BigBang.FluxPatchFiles = append(target.Extensions.BigBang.FluxPatchFiles, override.Extensions.BigBang.FluxPatchFiles...)
		}
	}

	// Merge deprecated scripts for backwards compatibility with older zarf binaries.
	target.DeprecatedScripts.Before = append(target.DeprecatedScripts.Before, override.DeprecatedScripts.Before...)
	target.DeprecatedScripts.After = append(target.DeprecatedScripts.After, override.DeprecatedScripts.After...)

	if override.DeprecatedScripts.Retry {
		target.DeprecatedScripts.Retry = true
	}
	if override.DeprecatedScripts.ShowOutput {
		target.DeprecatedScripts.ShowOutput = true
	}
	if override.DeprecatedScripts.TimeoutSeconds > 0 {
		target.DeprecatedScripts.TimeoutSeconds = override.DeprecatedScripts.TimeoutSeconds
	}

	// Merge create actions.
	target.Actions.OnCreate.Before = append(target.Actions.OnCreate.Before, override.Actions.OnCreate.Before...)
	target.Actions.OnCreate.After = append(target.Actions.OnCreate.After, override.Actions.OnCreate.After...)
	target.Actions.OnCreate.OnFailure = append(target.Actions.OnCreate.OnFailure, override.Actions.OnCreate.OnFailure...)
	target.Actions.OnCreate.OnSuccess = append(target.Actions.OnCreate.OnSuccess, override.Actions.OnCreate.OnSuccess...)

	// Merge deploy actions.
	target.Actions.OnDeploy.Before = append(target.Actions.OnDeploy.Before, override.Actions.OnDeploy.Before...)
	target.Actions.OnDeploy.After = append(target.Actions.OnDeploy.After, override.Actions.OnDeploy.After...)
	target.Actions.OnDeploy.OnFailure = append(target.Actions.OnDeploy.OnFailure, override.Actions.OnDeploy.OnFailure...)
	target.Actions.OnDeploy.OnSuccess = append(target.Actions.OnDeploy.OnSuccess, override.Actions.OnDeploy.OnSuccess...)

	// Merge remove actions.
	target.Actions.OnRemove.Before = append(target.Actions.OnRemove.Before, override.Actions.OnRemove.Before...)
	target.Actions.OnRemove.After = append(target.Actions.OnRemove.After, override.Actions.OnRemove.After...)
	target.Actions.OnRemove.OnFailure = append(target.Actions.OnRemove.OnFailure, override.Actions.OnRemove.OnFailure...)
	target.Actions.OnRemove.OnSuccess = append(target.Actions.OnRemove.OnSuccess, override.Actions.OnRemove.OnSuccess...)

	// Merge Only filters.
	target.Only.Cluster.Distros = append(target.Only.Cluster.Distros, override.Only.Cluster.Distros...)
	if override.Only.Cluster.Architecture != "" {
		target.Only.Cluster.Architecture = override.Only.Cluster.Architecture
	}
	if override.Only.LocalOS != "" {
		target.Only.LocalOS = override.Only.LocalOS
	}
}

// Prefix file path with importPath if original file path is not a url.
func (p *Packager) getComposedFilePath(prefix string, path string) string {
	// Return original if it is a remote file.
	if helpers.IsURL(path) {
		return path
	}

	// Add prefix for local files.
	return filepath.Join(prefix, path)
}
