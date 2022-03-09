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
		if !hasComposedPackage(&component) {
			// Append standard component to list.
			components = append(components, component)
		} else if shouldComposePackage(&component) { // Validate and confirm inclusion of imported package.
			// Expand and add components from imported package.
			importedComponents := getSubPackageAssets(component)
			components = append(components, importedComponents...)
		}
	}
	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	config.SetComponents(components)
	return components
}

// Returns true if import field is populated.
func hasComposedPackage(component *types.ZarfComponent) bool {
	return component.Import != types.ZarfImport{}
}

// Validates and confirms inclusion of imported package.
func shouldComposePackage(component *types.ZarfComponent) bool {
	validateOrBail(component)
	return componentConfirmedForInclusion(component)
}

// Returns true if confirm flag is true, the component is required, or the user confirms inclusion.
func componentConfirmedForInclusion(component *types.ZarfComponent) bool {
	return config.DeployOptions.Confirm || component.Required || ConfirmOptionalComponent(*component)
}

// Validates the sub component, exits program if validation fails.
func validateOrBail(component *types.ZarfComponent) {
	if err := validate.ValidateImportPackage(component); err != nil {
		message.Fatalf(err, "Invalid import definition in the %s component: %s", component.Name, err)
	}
}

// Get expanded components from imported component.
func getSubPackageAssets(importComponent types.ZarfComponent) (components []types.ZarfComponent) {
	// Read the imported package.
	importedPackage := getSubPackage(&importComponent)
	// Iterate imported components.
	for _, componentToCompose := range importedPackage.Components {
		// Check for standard component.
		if !hasComposedPackage(&componentToCompose) {
			// Doctor standard component name and included files.
			prepComponentToCompose(&componentToCompose, importedPackage.Metadata.Name, importComponent.Import.Path)
			components = append(components, componentToCompose)
		} else if shouldComposePackage(&componentToCompose) {
			// Recurse on imported components.
			components = append(components, getSubPackageAssets(componentToCompose)...)
		}
	}
	return components
}

// Reads the locally imported zarf.yaml
func getSubPackage(component *types.ZarfComponent) (importedPackage types.ZarfPackage) {
	utils.ReadYaml(component.Import.Path+"zarf.yaml", &importedPackage)
	return importedPackage
}

// Updates the name and sets all local asset paths relative to the importing package.
func prepComponentToCompose(component *types.ZarfComponent, parentPackageName string, importPath string) {
	// Prefix component name with parent package name to distinguish similarly named components.
	component.Name = parentPackageName + "-" + component.Name

	// Prefix composed component file paths.
	for fileIdx, file := range component.Files {
		component.Files[fileIdx].Source = getComposedFilePath(file.Source, importPath)
	}

	// Prefix non-url composed component chart values files.
	for chartIdx, chart := range component.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			component.Charts[chartIdx].ValuesFiles[valuesIdx] = getComposedFilePath(valuesFile, importPath)
		}
	}

	// Prefix non-url composed manifest files and kustomizations.
	for manifestIdx, manifest := range component.Manifests {
		for fileIdx, file := range manifest.Files {
			component.Manifests[manifestIdx].Files[fileIdx] = getComposedFilePath(file, importPath)
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			component.Manifests[manifestIdx].Kustomizations[kustomIdx] = getComposedFilePath(kustomization, importPath)
		}
	}
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
