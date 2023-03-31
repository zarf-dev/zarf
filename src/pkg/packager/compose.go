// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// composeComponents builds the composed components list for the current config.
func (p *Packager) composeComponents() error {
	message.Debugf("packager.ComposeComponents()")

	components := []types.ZarfComponent{}

	for _, component := range p.cfg.Pkg.Components {
		if component.Import.Path == "" {
			// Migrate any deprecated component configurations now
			component = deprecated.MigrateComponent(p.cfg.Pkg.Build, component)
			components = append(components, component)
		} else {
			composedComponent, err := p.getComposedComponent(component)
			if err != nil {
				return fmt.Errorf("unable to compose component %s: %w", component.Name, err)
			}
			components = append(components, composedComponent)
		}
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
	message.Debugf("packager.GetComposedComponent(%+v)", parentComponent)

	// Make sure the component we're trying to import can't be accessed.
	if err := validate.ImportPackage(&parentComponent); err != nil {
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
	message.Debugf("packager.getChildComponent(%+v, %s)", parent, pathAncestry)

	subPkg, err := p.getSubPackage(filepath.Join(pathAncestry, parent.Import.Path))
	if err != nil {
		return child, fmt.Errorf("unable to get sub package: %w", err)
	}

	// Figure out which component we are actually importing.
	// NOTE: Default to the component name if a custom one was not provided.
	childComponentName := parent.Import.ComponentName
	if childComponentName == "" {
		childComponentName = parent.Name
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

	// Check if we need to get more of children.
	if child.Import.Path != "" {
		// Set a temporary composePath so we can get future children/grandchildren from our current location.
		tmpPathAncestry := filepath.Join(pathAncestry, parent.Import.Path)

		// Recursively call this function to get the next layer of children.
		grandchildComponent, err := p.getChildComponent(child, tmpPathAncestry)
		if err != nil {
			return child, err
		}

		// Merge the grandchild values into the child.
		p.mergeComponentOverrides(&grandchildComponent, child)

		// Set the grandchild as the child component now that we're done with recursively importing.
		child = grandchildComponent
	}

	// Fix the filePaths of imported components to be accessible from our current location.
	child = p.fixComposedFilepaths(parent, child)

	// Migrate any deprecated component configurations now
	child = deprecated.MigrateComponent(p.cfg.Pkg.Build, child)

	return
}

func (p *Packager) fixComposedFilepaths(parent, child types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.fixComposedFilepaths(%+v, %+v)", child, parent)

	// Prefix composed component file paths.
	for fileIdx, file := range child.Files {
		child.Files[fileIdx].Source = p.getComposedFilePath(file.Source, parent.Import.Path)
	}

	// Prefix non-url composed component chart values files and localPath.
	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = p.getComposedFilePath(valuesFile, parent.Import.Path)
		}
		if child.Charts[chartIdx].LocalPath != "" {
			// Check if the localPath is relative to the parent Zarf package
			if _, err := os.Stat(child.Charts[chartIdx].LocalPath); os.IsNotExist(err) {
				// Since the chart localPath is not relative to the parent Zarf package, get the relative path from the composed child
				child.Charts[chartIdx].LocalPath = p.getComposedFilePath(child.Charts[chartIdx].LocalPath, parent.Import.Path)
			}
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			child.Manifests[manifestIdx].Files[fileIdx] = p.getComposedFilePath(file, parent.Import.Path)
		}
		for kustomizeIdx, kustomization := range manifest.Kustomizations {
			child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = p.getComposedFilePath(kustomization, parent.Import.Path)
		}
	}

	if child.CosignKeyPath != "" {
		child.CosignKeyPath = p.getComposedFilePath(child.CosignKeyPath, parent.Import.Path)
	}

	return child
}

// Sets Name, Default, Required and Description to the original components values.
func (p *Packager) mergeComponentOverrides(target *types.ZarfComponent, override types.ZarfComponent) {
	message.Debugf("packager.mergeComponentOverrides(%+v, %+v)", target, override)

	target.Name = override.Name
	target.Default = override.Default
	target.Required = override.Required
	target.Group = override.Group

	// Override description if it was provided.
	if override.Description != "" {
		target.Description = override.Description
	}

	// Override cosign key path if it was provided.
	if override.CosignKeyPath != "" {
		target.CosignKeyPath = override.CosignKeyPath
	}

	// Append slices where they exist.
	target.Charts = append(target.Charts, override.Charts...)
	target.DataInjections = append(target.DataInjections, override.DataInjections...)
	target.Files = append(target.Files, override.Files...)
	target.Images = append(target.Images, override.Images...)
	target.Manifests = append(target.Manifests, override.Manifests...)
	target.Repos = append(target.Repos, override.Repos...)
	// Check for nil array
	if override.Extensions.BigBang != nil {
		if override.Extensions.BigBang.ValuesFiles != nil {
			target.Extensions.BigBang.ValuesFiles = append(target.Extensions.BigBang.ValuesFiles, override.Extensions.BigBang.ValuesFiles...)
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

// Reads the locally imported zarf.yaml.
func (p *Packager) getSubPackage(packagePath string) (importedPackage types.ZarfPackage, err error) {
	message.Debugf("packager.getSubPackage(%s)", packagePath)

	path := filepath.Join(packagePath, config.ZarfYAML)
	if err := utils.ReadYaml(path, &importedPackage); err != nil {
		return importedPackage, err
	}

	// Merge in child package variables (only if the variable does not exist in parent).
	for _, importedVariable := range importedPackage.Variables {
		p.injectImportedVariable(importedVariable)
	}

	// Merge in child package constants (only if the constant does not exist in parent).
	for _, importedConstant := range importedPackage.Constants {
		p.injectImportedConstant(importedConstant)
	}

	return
}

// Prefix file path with importPath if original file path is not a url.
func (p *Packager) getComposedFilePath(originalPath string, pathPrefix string) string {
	message.Debugf("packager.getComposedFilePath(%s, %s)", originalPath, pathPrefix)

	// Return original if it is a remote file.
	if utils.IsURL(originalPath) {
		return originalPath
	}

	// Add prefix for local files.
	return filepath.Join(pathPrefix, originalPath)
}
