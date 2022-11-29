// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// composeComponents builds the composed components list for the current config.
func (p *Packager) composeComponents() error {
	message.Debugf("packager.ComposeComponents()")

	components := []types.ZarfComponent{}

	for _, component := range p.cfg.Pkg.Components {
		if component.Import.Path == "" {
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

// getComposedComponent recursively retrieves a composed zarf component
// --------------------------------------------------------------------
// For composed components, we build the tree of components starting at the root and adding children as we go;
// this follows the composite design pattern outlined here: https://en.wikipedia.org/wiki/Composite_pattern
// where 1 component parent is made up of 0...n composite or leaf children.
func (p *Packager) getComposedComponent(parentComponent types.ZarfComponent) (child types.ZarfComponent, err error) {
	message.Debugf("packager.GetComposedComponent(%+v)", parentComponent)

	// Make sure the component we're trying to import cant be accessed
	if err := validate.ImportPackage(&parentComponent); err != nil {
		return child, fmt.Errorf("invalid import definition in the %s component: %w", parentComponent.Name, err)
	}

	// Keep track of the composed components import path to build nestedily composed components
	pathAncestry := ""

	// Get the component that we are trying to import
	// NOTE: This function is recursive and will continue getting the children until there are no more 'imported' components left
	child, err = p.getChildComponent(parentComponent, pathAncestry)
	if err != nil {
		return child, fmt.Errorf("unable to get child component: %w", err)
	}

	// Merge the overrides from the child that we just received with the parent we were provided
	p.mergeComponentOverrides(&child, parentComponent)

	return
}

func (p *Packager) getChildComponent(parent types.ZarfComponent, pathAncestry string) (child types.ZarfComponent, err error) {
	message.Debugf("packager.getChildComponent(%+v, %s)", parent, pathAncestry)

	subPkg, err := p.getSubPackage(filepath.Join(pathAncestry, parent.Import.Path))
	if err != nil {
		return child, fmt.Errorf("unable to get sub package: %w", err)
	}

	// Figure out which component we are actually importing
	// NOTE: Default to the component name if a custom one was not provided
	childComponentName := parent.Import.ComponentName
	if childComponentName == "" {
		childComponentName = parent.Name
	}

	// Find the child component from the imported package that matches our arch
	for _, component := range subPkg.Components {
		if component.Name == childComponentName {
			filterArch := component.Only.Cluster.Architecture

			// Override the filter if it is set by the parent component
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

	// If we didn't find a child component, bail
	if child.Name == "" {
		return child, fmt.Errorf("unable to find the component %s in the imported package", childComponentName)
	}

	// Check if we need to get more of children
	if child.Import.Path != "" {
		// Set a temporary composePath so we can get future children/grandchildren from our current location
		tmpPathAncestry := filepath.Join(pathAncestry, parent.Import.Path)

		// Recursively call this function to get the next layer of children
		grandchildComponent, err := p.getChildComponent(child, tmpPathAncestry)
		if err != nil {
			return child, err
		}

		// Merge the grandchild values into the child
		p.mergeComponentOverrides(&grandchildComponent, child)

		// Set the grandchild as the child component now that we're done with recursively importing
		child = grandchildComponent
	}

	// Fix the filePaths of imported components to be accessible from our current location
	child = p.fixComposedFilepaths(parent, child)

	return
}

func (p *Packager) fixComposedFilepaths(parent, child types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.fixComposedFilepaths(%+v, %+v)", child, parent)

	// Prefix composed component file paths.
	for fileIdx, file := range child.Files {
		child.Files[fileIdx].Source = p.getComposedFilePath(file.Source, parent.Import.Path)
	}

	// Prefix non-url composed component chart values files.
	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = p.getComposedFilePath(valuesFile, parent.Import.Path)
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			child.Manifests[manifestIdx].Files[fileIdx] = p.getComposedFilePath(file, parent.Import.Path)
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			child.Manifests[manifestIdx].Kustomizations[kustomIdx] = p.getComposedFilePath(kustomization, parent.Import.Path)
		}
	}

	if child.CosignKeyPath != "" {
		child.CosignKeyPath = p.getComposedFilePath(child.CosignKeyPath, parent.Import.Path)
	}

	return child
}

// Sets Name, Default, Required and Description to the original components values
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

	// Merge scripts.
	target.Scripts.Before = append(target.Scripts.Before, override.Scripts.Before...)
	target.Scripts.After = append(target.Scripts.After, override.Scripts.After...)

	if override.Scripts.Retry {
		target.Scripts.Retry = true
	}
	if override.Scripts.ShowOutput {
		target.Scripts.ShowOutput = true
	}
	if override.Scripts.TimeoutSeconds > 0 {
		target.Scripts.TimeoutSeconds = override.Scripts.TimeoutSeconds
	}

	// Merge Only filters
	target.Only.Cluster.Distros = append(target.Only.Cluster.Distros, override.Only.Cluster.Distros...)
	if override.Only.Cluster.Architecture != "" {
		target.Only.Cluster.Architecture = override.Only.Cluster.Architecture
	}
	if override.Only.LocalOS != "" {
		target.Only.LocalOS = override.Only.LocalOS
	}
}

// Reads the locally imported zarf.yaml
func (p *Packager) getSubPackage(packagePath string) (importedPackage types.ZarfPackage, err error) {
	message.Debugf("packager.getSubPackage(%s)", packagePath)

	path := filepath.Join(packagePath, config.ZarfYAML)
	if err := utils.ReadYaml(path, &importedPackage); err != nil {
		return importedPackage, err
	}

	// Merge in child package variables (only if the variable does not exist in parent)
	for _, importedVariable := range importedPackage.Variables {
		p.injectImportedVariable(importedVariable)
	}

	// Merge in child package constants (only if the constant does not exist in parent)
	for _, importedConstant := range importedPackage.Constants {
		p.injectImportedConstant(importedConstant)
	}

	return
}

// Prefix file path with importPath if original file path is not a url.
func (p *Packager) getComposedFilePath(originalPath string, pathPrefix string) string {
	message.Debugf("packager.getComposedFilePath(%s, %s)", originalPath, pathPrefix)

	// Return original if it is a remote file.
	if utils.IsUrl(originalPath) {
		return originalPath
	}

	// Add prefix for local files.
	return filepath.Join(pathPrefix, originalPath)
}
