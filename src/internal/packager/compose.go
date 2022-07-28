package packager

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// ComposeComponents builds the composed components list for the current config.
func ComposeComponents() {
	message.Debugf("packager.ComposeComponents()")

	components := []types.ZarfComponent{}

	for _, component := range config.GetComponents() {
		if component.Import.Path == "" {
			components = append(components, component)
		} else {
			components = append(components, GetComposedComponent(component))
		}
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	config.SetComponents(components)
}

// GetComposedComponent recursively retrieves a composed zarf component
// --------------------------------------------------------------------
// For composed components, we build the tree of components starting at the root and adding children as we go;
// this follows the composite design pattern outlined here: https://en.wikipedia.org/wiki/Composite_pattern
// where 1 component parent is made up of 0...n composite or leaf children.
func GetComposedComponent(parentComponent types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.GetComposedComponent(%+v)", parentComponent)

	// Make sure the component we're trying to import cant be accessed
	validateOrBail(&parentComponent)

	// Keep track of the composed components import path to build nestedily composed components
	everGrowingComposePath := ""

	// Get the component that we are trying to import
	// NOTE: This function is recursive and will continue getting the children until there are no more 'imported' components left
	childComponent := getChildComponent(parentComponent, everGrowingComposePath)

	// Merge the overrides from the child that we just received with the parent we were provided
	mergeComponentOverrides(&childComponent, parentComponent)

	return childComponent
}

func getChildComponent(parentComponent types.ZarfComponent, everGrowingComposePath string) (childComponent types.ZarfComponent) {
	message.Debugf("packager.getChildComponent(%+v, %s)", parentComponent, everGrowingComposePath)

	importedPackage := getSubPackage(filepath.Join(everGrowingComposePath, parentComponent.Import.Path))

	// Figure out which component we are actually importing
	// NOTE: Default to the component name if a custom one was not provided
	childComponentName := parentComponent.Import.ComponentName
	if childComponentName == "" {
		childComponentName = parentComponent.Name
	}

	targetArch := config.GetArch()
	// Find the child component from the imported package that matches our arch
	for _, importedComponent := range importedPackage.Components {
		if importedComponent.Name == childComponentName {
			filterArch := importedComponent.Only.Cluster.Architecture

			// Override the filter if it is set by the parent component
			if parentComponent.Only.Cluster.Architecture != "" {
				filterArch = parentComponent.Only.Cluster.Architecture
			}

			// Only add this component if it is valid for the target architecture.
			if filterArch == "" || filterArch == targetArch {
				childComponent = importedComponent
				break
			}
		}
	}

	// If we didn't find a child component, bail
	if childComponent.Name == "" {
		message.Fatalf(nil, "Unable to find the component %s in the imported package", childComponentName)
	}

	// Check if we need to get more of children
	if childComponent.Import.Path != "" {
		// Set a temporary composePath so we can get future children/grandchildren from our current location
		tempEverGrowingComposePath := filepath.Join(everGrowingComposePath, parentComponent.Import.Path)

		// Recursively call this function to get the next layer of children
		grandchildComponent := getChildComponent(childComponent, tempEverGrowingComposePath)

		// Merge the grandchild values into the child
		mergeComponentOverrides(&grandchildComponent, childComponent)

		// Set the grandchild as the child component now that we're done with recursively importing
		childComponent = grandchildComponent
	}

	// Fix the filePaths of imported components to be accessible from our current location
	childComponent = fixComposedFilepaths(parentComponent, childComponent)

	return
}

func fixComposedFilepaths(parentComponent, childComponent types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.fixComposedFilepaths(%+v, %+v)", childComponent, parentComponent)

	// Prefix composed component file paths.
	for fileIdx, file := range childComponent.Files {
		childComponent.Files[fileIdx].Source = getComposedFilePath(file.Source, parentComponent.Import.Path)
	}

	// Prefix non-url composed component chart values files.
	for chartIdx, chart := range childComponent.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			childComponent.Charts[chartIdx].ValuesFiles[valuesIdx] = getComposedFilePath(valuesFile, parentComponent.Import.Path)
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range childComponent.Manifests {
		for fileIdx, file := range manifest.Files {
			childComponent.Manifests[manifestIdx].Files[fileIdx] = getComposedFilePath(file, parentComponent.Import.Path)
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			childComponent.Manifests[manifestIdx].Kustomizations[kustomIdx] = getComposedFilePath(kustomization, parentComponent.Import.Path)
		}
	}

	if childComponent.CosignKeyPath != "" {
		childComponent.CosignKeyPath = getComposedFilePath(childComponent.CosignKeyPath, parentComponent.Import.Path)
	}

	return childComponent
}

// Validates the sub component, exits program if validation fails.
func validateOrBail(component *types.ZarfComponent) {
	message.Debugf("packager.validateOrBail(%+v)", component)

	if err := validate.ValidateImportPackage(component); err != nil {
		message.Fatalf(err, "Invalid import definition in the %s component: %s", component.Name, err)
	}
}

// Sets Name, Default, Required and Description to the original components values
func mergeComponentOverrides(target *types.ZarfComponent, override types.ZarfComponent) {
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
func getSubPackage(packagePath string) (importedPackage types.ZarfPackage) {
	message.Debugf("packager.getSubPackage(%s)", packagePath)

	path := filepath.Join(packagePath, config.ZarfYAML)
	if err := utils.ReadYaml(path, &importedPackage); err != nil {
		message.Fatalf(err, "Unable to read the %s file", path)
	}

	// Merge in child package variables (only if the variable does not exist in parent)
	for _, importedVariable := range importedPackage.Variables {
		config.InjectImportedVariable(importedVariable)
	}

	// Merge in child package constants (only if the constant does not exist in parent)
	for _, importedConstant := range importedPackage.Constants {
		config.InjectImportedConstant(importedConstant)
	}

	return importedPackage
}

// Prefix file path with importPath if original file path is not a url.
func getComposedFilePath(originalPath string, pathPrefix string) string {
	message.Debugf("packager.getComposedFilePath(%s, %s)", originalPath, pathPrefix)

	// Return original if it is a remote file.
	if utils.IsUrl(originalPath) {
		return originalPath
	}

	// Add prefix for local files.
	return filepath.Join(pathPrefix, originalPath)
}
