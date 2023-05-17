// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

// composeComponents builds the composed components list for the current config.
func (p *Packager) composeComponents() error {
	message.Debugf("packager.ComposeComponents()")

	components := []types.ZarfComponent{}

	for _, component := range p.cfg.Pkg.Components {
		if component.Import.Path == "" && component.Import.URL == "" {
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

	// Figure out which component we are actually importing.
	// NOTE: Default to the component name if a custom one was not provided.
	childComponentName := parent.Import.ComponentName
	if childComponentName == "" {
		childComponentName = parent.Name
	}

	var cachePath string
	if parent.Import.URL != "" {
		skelURL := strings.TrimPrefix(parent.Import.URL, utils.OCIURLPrefix)
		cachePath = filepath.Join(config.GetAbsCachePath(), "oci", skelURL)
		err = os.MkdirAll(cachePath, 0755)
		if err != nil {
			return child, fmt.Errorf("unable to create cache path %s: %w", cachePath, err)
		}

		componentLayer := filepath.Join("components", fmt.Sprintf("%s.tar", childComponentName))
		err = p.handleOciPackage(skelURL, cachePath, 3, componentLayer)
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
	}

	subPkg, err := p.getSubPackage(filepath.Join(pathAncestry, parent.Import.Path))
	if err != nil {
		return child, fmt.Errorf("unable to get sub package: %w", err)
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
		dir := filepath.Join(cachePath, "components", child.Name)
		parent.Import.Path = filepath.Join(parent.Import.Path, "components", child.Name)
		if !utils.InvalidPath(dir) {
			err = os.RemoveAll(dir)
			if err != nil {
				return child, fmt.Errorf("unable to remove composed component cache path %s: %w", cachePath, err)
			}
		}
		err = archiver.Unarchive(fmt.Sprintf("%s.tar", dir), filepath.Join(cachePath, "components"))
		if err != nil {
			return child, fmt.Errorf("unable to unpack composed component tarball: %w", err)
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
	child = deprecated.MigrateComponent(p.cfg.Pkg.Build, child)

	return
}

func (p *Packager) fixComposedFilepaths(pathAncestry string, child types.ZarfComponent) (types.ZarfComponent, error) {
	message.Debugf("packager.fixComposedFilepaths(%+v, %+v)", pathAncestry, child)

	for fileIdx, file := range child.Files {
		composed, err := p.getComposedFilePath(pathAncestry, file.Source)
		if err != nil {
			return child, err
		}
		child.Files[fileIdx].Source = composed
	}

	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			composed, err := p.getComposedFilePath(pathAncestry, valuesFile)
			if err != nil {
				return child, err
			}
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = composed
		}
		if child.Charts[chartIdx].LocalPath != "" {
			composed, err := p.getComposedFilePath(pathAncestry, child.Charts[chartIdx].LocalPath)
			if err != nil {
				return child, err
			}
			child.Charts[chartIdx].LocalPath = composed
		}
	}

	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			composed, err := p.getComposedFilePath(pathAncestry, file)
			if err != nil {
				return child, err
			}
			child.Manifests[manifestIdx].Files[fileIdx] = composed
		}
		for kustomizeIdx, kustomization := range manifest.Kustomizations {
			composed, err := p.getComposedFilePath(pathAncestry, kustomization)
			if err != nil {
				return child, err
			}
			// kustomizations can use non-standard urls, so we need to check if the composed path exists on the local filesystem
			abs, _ := filepath.Abs(composed)
			invalid := utils.InvalidPath(abs)
			if !invalid {
				child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = composed
			}
		}
	}

	for dataInjectionsIdx, dataInjection := range child.DataInjections {
		composed, err := p.getComposedFilePath(pathAncestry, dataInjection.Source)
		if err != nil {
			return child, err
		}
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
		composedDefaultDir, err := p.getComposedFilePath(pathAncestry, child.Actions.OnCreate.Defaults.Dir)
		if err != nil {
			return child, err
		}
		child.Actions.OnCreate.Defaults.Dir = composedDefaultDir
	}

	if child.CosignKeyPath != "" {
		composed, err := p.getComposedFilePath(pathAncestry, child.CosignKeyPath)
		if err != nil {
			return child, err
		}
		child.CosignKeyPath = composed
	}

	return child, nil
}

func (p *Packager) fixComposedActionFilepaths(pathAncestry string, actions []types.ZarfComponentAction) ([]types.ZarfComponentAction, error) {
	for actionIdx, action := range actions {
		if action.Dir != nil {
			composedActionDir, err := p.getComposedFilePath(pathAncestry, *action.Dir)
			if err != nil {
				return actions, err
			}
			actions[actionIdx].Dir = &composedActionDir
		}
	}

	return actions, nil
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
func (p *Packager) getComposedFilePath(prefix string, path string) (string, error) {
	message.Debugf("packager.getComposedFilePath(%s, %s)", prefix, path)

	// Return original if it is a remote file.
	if utils.IsURL(path) {
		return path, nil
	}

	// Add prefix for local files.
	relativeToParent := filepath.Join(prefix, path)

	abs, err := filepath.Abs(relativeToParent)
	if err != nil {
		return "", err
	}
	if utils.InvalidPath(abs) {
		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		if !utils.InvalidPath(pathAbs) {
			return "", fmt.Errorf("imported path %s does not exist, please update %s to be relative to the imported component", relativeToParent, path)
		}
		return "", fmt.Errorf("imported path %s does not exist", relativeToParent)
	}

	return relativeToParent, nil
}
