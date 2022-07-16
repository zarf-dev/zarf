package packager

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func GetComponents() (components []types.ZarfComponent) {
	message.Debugf("packager.GetComponents()")

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

	return components
}

func GetComposedComponent(childComponent types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.GetComposedComponent(%+v)", childComponent)

	// Make sure the component we're trying to import cant be accessed
	validateOrBail(&childComponent)

	// Keep track of the composed components import path to build nestedily composed components
	everGrowingComposePath := ""

	// Get the component that we are trying to import
	// NOTE: This function is recursive and will continue getting the parents until there are no more 'imported' components left
	parentComponent := getParentComponent(childComponent, everGrowingComposePath)

	// Merge the overrides from the parent that we just received with the child we were provided
	mergeComponentOverrides(&parentComponent, childComponent)

	return parentComponent
}

func getParentComponent(childComponent types.ZarfComponent, everGrowingComposePath string) (parentComponent types.ZarfComponent) {
	message.Debugf("packager.getParentComponent(%+v, %s)", childComponent, everGrowingComposePath)

	importedPackage, err := getSubPackage(filepath.Join(everGrowingComposePath, childComponent.Import.Path))
	if err != nil {
		message.Fatal(err, "Unable to get the package that we're importing a component from")
	}

	// Figure out which component we are actually importing
	// NOTE: Default to the component name if a custom one was not provided
	parentComponentName := childComponent.Import.ComponentName
	if parentComponentName == "" {
		parentComponentName = childComponent.Name
	}

	targetArch := config.GetArch()
	// Find the parent component from the imported package that matches our arch
	for _, importedComponent := range importedPackage.Components {
		if importedComponent.Name == parentComponentName {
			filterArch := importedComponent.Only.Cluster.Architecture

			// Override the filter if it is set by the child component
			if childComponent.Only.Cluster.Architecture != "" {
				filterArch = childComponent.Only.Cluster.Architecture
			}

			// Only add this component if it is valid for the target architecture.
			if filterArch == "" || filterArch == targetArch {
				parentComponent = importedComponent
				break
			}
		}
	}

	// If we didn't find a parent component, bail
	if parentComponent.Name == "" {
		message.Fatalf(nil, "Unable to find the component %s in the imported package", parentComponentName)
	}

	// Check if we need to get more of the parents!!!
	if parentComponent.Import.Path != "" {
		// Set a temporary composePath so we can get future parents/grandparents from our current location
		tempEverGrowingComposePath := filepath.Join(everGrowingComposePath, childComponent.Import.Path)

		// Recursively call this function to get the next layer of parents
		grandparentComponent := getParentComponent(parentComponent, tempEverGrowingComposePath)

		// Merge the grandparents values into the parent
		mergeComponentOverrides(&grandparentComponent, parentComponent)

		// Set the grandparent as the parent component now that we're done with recursively importing
		parentComponent = grandparentComponent
	}

	// Fix the filePaths of imported components to be accessible from our current location
	parentComponent = fixComposedFilepaths(parentComponent, childComponent)

	return
}

func fixComposedFilepaths(parentComponent, childComponent types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.fixComposedFilepaths(%+v, %+v)", parentComponent, childComponent)

	// Prefix composed component file paths.
	for fileIdx, file := range parentComponent.Files {
		parentComponent.Files[fileIdx].Source = getComposedFilePath(file.Source, childComponent.Import.Path)
	}

	// Prefix non-url composed component chart values files.
	for chartIdx, chart := range parentComponent.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			parentComponent.Charts[chartIdx].ValuesFiles[valuesIdx] = getComposedFilePath(valuesFile, childComponent.Import.Path)
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range parentComponent.Manifests {
		for fileIdx, file := range manifest.Files {
			parentComponent.Manifests[manifestIdx].Files[fileIdx] = getComposedFilePath(file, childComponent.Import.Path)
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			parentComponent.Manifests[manifestIdx].Kustomizations[kustomIdx] = getComposedFilePath(kustomization, childComponent.Import.Path)
		}
	}

	if parentComponent.CosignKeyPath != "" {
		parentComponent.CosignKeyPath = getComposedFilePath(parentComponent.CosignKeyPath, childComponent.Import.Path)
	}

	return parentComponent
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

	// Merge variables.
	for key, variable := range override.Variables {
		target.Variables[key] = variable
	}

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
func getSubPackage(packagePath string) (importedPackage types.ZarfPackage, err error) {
	message.Debugf("packager.getSubPackage(%s)", packagePath)

	path := filepath.Join(packagePath, config.ZarfYAML)
	err = utils.ReadYaml(path, &importedPackage)
	return importedPackage, err
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
