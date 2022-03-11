package packager

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/packager/validate"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/defenseunicorns/zarf/cli/types"
)

func GetComposedComponents() (components []types.ZarfComponent) {
	for _, component := range config.GetComponents() {
		// Check for standard component.
		if component.Import.Path == "" {
			// Append standard component to list.
			components = append(components, component)
		} else {
			validateOrBail(&component)

			// Expand and add components from imported package.
			importedComponent := getSubPackageAssets(component)
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

// Get expanded components from imported component.
func getSubPackageAssets(importComponent types.ZarfComponent) (component types.ZarfComponent) {
	// Read the imported package.
	importedPackage := getSubPackage(&importComponent)

	for _, componentToCompose := range importedPackage.Components {
		if componentToCompose.Name == importComponent.Import.ComponentName {
			return *prepComponentToCompose(&componentToCompose, importComponent)
		}
	}

	return component
}

// Reads the locally imported zarf.yaml
func getSubPackage(component *types.ZarfComponent) (importedPackage types.ZarfPackage) {
	utils.ReadYaml(component.Import.Path+"zarf.yaml", &importedPackage)
	return importedPackage
}

// Updates the name and sets all local asset paths relative to the importing component.
func prepComponentToCompose(componentToCompose *types.ZarfComponent, importComponent types.ZarfComponent) *types.ZarfComponent {

	if componentToCompose.Import.Path != "" {
		// The component we are trying to compose is a composed component itself!
		nestedComponent := getSubPackageAssets(*componentToCompose)
		componentToCompose = prepComponentToCompose(&nestedComponent, *componentToCompose)
	}

	componentToCompose.Name = importComponent.Name

	// Prefix composed component file paths.
	for fileIdx, file := range componentToCompose.Files {
		componentToCompose.Files[fileIdx].Source = getComposedFilePath(file.Source, importComponent.Import.Path)
	}

	// Prefix non-url composed component chart values files.
	for chartIdx, chart := range componentToCompose.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			componentToCompose.Charts[chartIdx].ValuesFiles[valuesIdx] = getComposedFilePath(valuesFile, importComponent.Import.Path)
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range componentToCompose.Manifests {
		for fileIdx, file := range manifest.Files {
			componentToCompose.Manifests[manifestIdx].Files[fileIdx] = getComposedFilePath(file, importComponent.Import.Path)
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			componentToCompose.Manifests[manifestIdx].Kustomizations[kustomIdx] = getComposedFilePath(kustomization, importComponent.Import.Path)
		}
	}

	return componentToCompose
}

// Prefix file path with importPath if original file path is not a url.
func getComposedFilePath(originalPath string, pathPrefix string) string {
	// Return original if it is a remote file.
	if utils.IsUrl(originalPath) {
		return originalPath
	}
	// Add prefix for local files.
	return pathPrefix + originalPath
}
