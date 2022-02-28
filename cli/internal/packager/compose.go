package packager

import (
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/defenseunicorns/zarf/cli/types"
)

func GetComposedAssets() (components []types.ZarfComponent, seedImages []string) {
	for _, component := range config.GetComponents() {
		// Build components list by expanding imported components.
		if hasSubPackage(&component) {
			importedComponents, importedImages := getSubPackageAssets(component)
			components = append(components, importedComponents...)
			seedImages = append(seedImages, importedImages...)

		} else {
			components = append(components, component)
		}
	}
	// Update the parent package config with the expanded sub components and seed images.
	// This is important when the deploy package is created.
	config.SetComponents(components)
	config.SetSeedImages(seedImages)
	return components, seedImages
}

// Get the sub package components/seed images to add to parent assets, recurses on sub imports.
func getSubPackageAssets(importComponent types.ZarfComponent) (components []types.ZarfComponent, seedImages []string) {
	importedPackage := getSubPackage(&importComponent)
	seedImages = importedPackage.Seed
	for _, componentToCompose := range importedPackage.Components {
		if hasSubPackage(&componentToCompose) {
			subComp, subImage := getSubPackageAssets(componentToCompose)
			components = append(components, subComp...)
			seedImages = append(seedImages, subImage...)
		} else {
			prepComponentToCompose(&componentToCompose, importedPackage.Metadata.Name, importComponent.Import.Path)
			components = append(components, componentToCompose)
		}
	}
	return components, seedImages
}

// Confirms inclusion of SubPackage. Need team input.
func shouldAddImportedPackage(component *types.ZarfComponent) bool {
	return hasSubPackage(component) && (component.Required || ConfirmOptionalComponent(*component))
}

// returns true if import has url
func hasSubPackage(component *types.ZarfComponent) bool {
	return len(component.Import.Path) > 0
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
	for idx, file := range component.Files {
		if !utils.IsUrl(file.Source) {
			component.Files[idx].Source = importPath + file.Source
		}
	}

	// Add import path to local chart values files.
	for chartIndex, chart := range component.Charts {
		for valuesIndex, valuesFile := range chart.ValuesFiles {
			if !utils.IsUrl(valuesFile) {
				component.Charts[chartIndex].ValuesFiles[valuesIndex] = importPath + valuesFile
			}
		}
	}

	// Add import path to local manifest files and kustomizations
	for manifestIndex, manifest := range component.Manifests {
		for fileIndex, file := range manifest.Files {
			if !utils.IsUrl(file) {
				component.Manifests[manifestIndex].Files[fileIndex] = importPath + file
			}
		}
		for kustomizationIndex, kustomization := range manifest.Kustomizations {
			if !utils.IsUrl(kustomization) {
				component.Manifests[manifestIndex].Kustomizations[kustomizationIndex] = importPath + kustomization
			}
		}
	}
}
