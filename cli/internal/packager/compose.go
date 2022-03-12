package packager

import (
	"strings"

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
			importedComponent := getImportedComponent(component)
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

// Sets Name, Default, Required, Description and SecretName to the original components values
func mergeComponentOverrides(target *types.ZarfComponent, src types.ZarfComponent) {
	target.Name = src.Name
	target.Default = src.Default
	target.Required = src.Required

	if src.Description != "" {
		target.Description = src.Description
	}

	if src.SecretName != "" {
		target.SecretName = src.SecretName
	}
}

// Get expanded components from imported component.
func getImportedComponent(importComponent types.ZarfComponent) (component types.ZarfComponent) {
	// Read the imported package.
	importedPackage := getSubPackage(&importComponent)

	componentName := importComponent.Import.ComponentName
	// Default to the component name if a custom one was not provided
	if componentName == "" {
		componentName = importComponent.Name
	}

	// Loop over package components looking for a match the componentName
	for _, componentToCompose := range importedPackage.Components {
		if componentToCompose.Name == componentName {
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
func prepComponentToCompose(child *types.ZarfComponent, parent types.ZarfComponent) *types.ZarfComponent {

	if child.Import.Path != "" {
		// The component we are trying to compose is a composed component itself!
		nestedComponent := getImportedComponent(*child)
		child = prepComponentToCompose(&nestedComponent, *child)
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

	return child
}

// Prefix file path with importPath if original file path is not a url.
func getComposedFilePath(originalPath string, pathPrefix string) string {
	// Return original if it is a remote file.
	if utils.IsUrl(originalPath) {
		return originalPath
	}
	// Add prefix for local files.
	return fixRelativePathBacktracking(pathPrefix + originalPath)
}

func fixRelativePathBacktracking(path string) string {
	var newPathBuilder []string
	var hitRealPath = false // We might need to go back several directories at the begining

	// Turn paths like `../../this/is/a/very/../silly/../path` into `../../this/is/a/path`
	splitString := strings.Split(path, "/")
	for _, dir := range splitString {
		if dir == ".." {
			if hitRealPath {
				// Instead of going back a directory, just don't get here in the first place
				newPathBuilder = newPathBuilder[:len(newPathBuilder)-1]
			} else {
				// We are still going back directories for the first time, keep going back
				newPathBuilder = append(newPathBuilder, dir)
			}
		} else {
			// This is a regular directory we want to travel through
			hitRealPath = true
			newPathBuilder = append(newPathBuilder, dir)
		}
	}

	// NOTE: This assumes a relative path
	return strings.Join(newPathBuilder, "/")
}
