package packager

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

func GetComposedComponents() (components []types.ZarfComponent) {
	for _, component := range config.GetComponents() {
		// Check for standard component.
		if component.Import.Path == "" {
			// Append standard component to list.
			components = append(components, component)
		} else {
			validateOrBail(&component)

			// Track the composed components import path to build nestedily composed components
			everGrowingComposePath := ""

			// Expand and add components from imported package.
			importedComponent := getImportedComponent(component, everGrowingComposePath)

			// Merge in parent component changes.
			mergeComponentOverrides(&importedComponent, component)
			// Add to the list of components for the package.
			components = append(components, importedComponent)
		}
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	config.SetComponents(components)
	return components
}

// Validates the sub component, exits program if validation fails.
func validateOrBail(component *types.ZarfComponent) {
	if err := validate.ValidateImportPackage(component); err != nil {
		message.Fatalf(err, "Invalid import definition in the %s component: %s", component.Name, err)
	}
}

// Sets Name, Default, Required and Description to the original components values
func mergeComponentOverrides(target *types.ZarfComponent, override types.ZarfComponent) {
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
	target.Scripts.ShowOutput = override.Scripts.ShowOutput
	if override.Scripts.TimeoutSeconds > 0 {
		target.Scripts.TimeoutSeconds = override.Scripts.TimeoutSeconds
	}

	// Merge Only filters
	target.Only.Architectures = append(target.Only.Architectures, override.Only.Architectures...)
	target.Only.OperatingSystems = append(target.Only.OperatingSystems, override.Only.OperatingSystems...)
}

// Get expanded components from imported component.
func getImportedComponent(importComponent types.ZarfComponent, everGrowingComposePath string) (component types.ZarfComponent) {

	// Read the imported package.
	importedPackage, err := getSubPackage(&importComponent, everGrowingComposePath)
	if err != nil {
		message.Fatal(err, "Unable to get the package we're importing a component from")
	}
	componentName := importComponent.Import.ComponentName
	// Default to the component name if a custom one was not provided
	if componentName == "" {
		componentName = importComponent.Name
	}

	// Loop over package components looking for a match the componentName
	for _, componentToCompose := range importedPackage.Components {
		if componentToCompose.Name == componentName {
			everGrowingComposePath = filepath.Join(everGrowingComposePath, importComponent.Import.Path)
			return *prepComponentToCompose(&componentToCompose, importComponent, everGrowingComposePath)
		}
	}

	return component
}

// Reads the locally imported zarf.yaml
func getSubPackage(component *types.ZarfComponent, everGrowingComposePath string) (importedPackage types.ZarfPackage, err error) {
	everGrowingComposePath = filepath.Join(everGrowingComposePath, component.Import.Path)

	path := filepath.Join(everGrowingComposePath, config.ZarfYAML)
	err = utils.ReadYaml(path, &importedPackage)
	return importedPackage, err
}

// Updates the name and sets all local asset paths relative to the importing component.
func prepComponentToCompose(child *types.ZarfComponent, parent types.ZarfComponent, everGrowingPath string) *types.ZarfComponent {
	if child.Import.Path != "" {
		// The component we are trying to compose is a composed component itself!
		nestedComponent := getImportedComponent(*child, everGrowingPath)

		mergeComponentOverrides(child, nestedComponent)

		if nestedComponent.Import.Path != "" {
			// Keep going down the import paths until we reach the end via recursion
			everGrowingPath = filepath.Join(everGrowingPath, nestedComponent.Import.Path)
			prepComponentToCompose(&nestedComponent, *child, everGrowingPath)
		}
	}

	// Prefix composed component file paths.
	for fileIdx, file := range child.Files {
		child.Files[fileIdx].Source = getComposedFilePath(file.Source, parent.Import.Path)
	}

	// Prefix non-url composed component chart values files.
	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = getComposedFilePath(valuesFile, parent.Import.Path)
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			child.Manifests[manifestIdx].Files[fileIdx] = getComposedFilePath(file, parent.Import.Path)
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			child.Manifests[manifestIdx].Kustomizations[kustomIdx] = getComposedFilePath(kustomization, parent.Import.Path)
		}
	}

	if child.CosignKeyPath != "" {
		child.CosignKeyPath = getComposedFilePath(child.CosignKeyPath, parent.Import.Path)
	}
	return child
}

// Prefix file path with importPath if original file path is not a url.
func getComposedFilePath(originalPath string, pathPrefix string) string {
	// Return original if it is a remote file.
	if utils.IsUrl(originalPath) {
		return originalPath
	}

	// Add prefix for local files.
	return filepath.Join(pathPrefix, originalPath)
}
