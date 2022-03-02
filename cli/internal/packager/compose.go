package packager

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/packager/validate"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/defenseunicorns/zarf/cli/types"
)

func GetComposedAssets() (components []types.ZarfComponent) {
	for _, component := range config.GetComponents() {
		// Build components list by expanding imported components.
		if shouldAddImportedPackage(&component) {
			importedComponents := getSubPackageAssets(component)
			components = append(components, importedComponents...)

		} else {
			components = append(components, component)
		}
	}
	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	config.SetComponents(components)
	return components
}

// Get the sub package components to add to parent assets, recurses on sub imports.
func getSubPackageAssets(importComponent types.ZarfComponent) (components []types.ZarfComponent) {
	importedPackage := getSubPackage(&importComponent)
	for _, componentToCompose := range importedPackage.Components {
		if shouldAddImportedPackage(&componentToCompose) {
			components = append(components, getSubPackageAssets(componentToCompose)...)
		} else {
			prepComponentToCompose(&componentToCompose, importedPackage.Metadata.Name, importComponent.Import.Path)
			components = append(components, componentToCompose)
		}
	}
	return components
}

// Confirms inclusion of SubPackage. Need team input.
func shouldAddImportedPackage(component *types.ZarfComponent) bool {
	return hasValidSubPackage(component) && (config.DeployOptions.Confirm || component.Required || ConfirmOptionalComponent(*component))
}

// Validates the sub component, errors out if validation fails.
func hasValidSubPackage(component *types.ZarfComponent) bool {
	if !hasSubPackage(component) {
		return false
	}
	if err := validate.ValidateImportPackage(component); err != nil {
		message.Fatalf(err, "Invalid import definition in the %s component: %s", component.Name, err)
	}
	return true
}

// returns true if import field is populated
func hasSubPackage(component *types.ZarfComponent) bool {
	return component.Import != types.ZarfImport{}
}

// Reads the locally imported zarf.yaml
func getSubPackage(component *types.ZarfComponent) (importedPackage types.ZarfPackage) {
	utils.ReadYaml(component.Import.Path+"zarf.yaml", &importedPackage)
	return importedPackage
}

// Updates the name and sets all local asset paths relative to the importing package.
func prepComponentToCompose(component *types.ZarfComponent, parentPackageName string, importPath string) {
	component.Name = parentPackageName + "-" + component.Name

	// Add import path to local component files.
	for fileIdx, file := range component.Files {
		if !utils.IsUrl(file.Source) {
			component.Files[fileIdx].Source = importPath + file.Source
		}
	}

	// Add import path to local chart values files.
	for chartIdx, chart := range component.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			if !utils.IsUrl(valuesFile) {
				component.Charts[chartIdx].ValuesFiles[valuesIdx] = importPath + valuesFile
			}
		}
	}

	// Add import path to local manifest files and kustomizations
	for manifestIdx, manifest := range component.Manifests {
		for fileIdx, file := range manifest.Files {
			if !utils.IsUrl(file) {
				component.Manifests[manifestIdx].Files[fileIdx] = importPath + file
			}
		}
		for kustomIdx, kustomization := range manifest.Kustomizations {
			if !utils.IsUrl(kustomization) {
				component.Manifests[manifestIdx].Kustomizations[kustomIdx] = importPath + kustomization
			}
		}
	}
}
