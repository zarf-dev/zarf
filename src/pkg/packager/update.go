// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// UpdateSchema updates the values.schema field in a zarf.yaml to point to the given relative schema filename.
func UpdateSchema(ctx context.Context, packagePath string, schemaFilename string) error {
	l := logger.From(ctx)
	return modifyManifest(packagePath, func(zarfPackage v1alpha1.ZarfPackage, astFile *ast.File, manifestPath string) (bool, error) {
		if err := createSchemaUpdate(zarfPackage, schemaFilename, astFile); err != nil {
			return false, fmt.Errorf("failed to create update: %w", err)
		}
		l.Info("successfully updated schema path", "path", manifestPath)
		return true, nil
	})
}

// UpdateImages updates the images field for components in a zarf.yaml.
func UpdateImages(ctx context.Context, packagePath string, definitionImageResults []DefinitionImageResult) error {
	l := logger.From(ctx)
	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return err
	}
	var header struct {
		APIVersion string `json:"apiVersion"`
	}
	if err := yaml.Unmarshal(contents, &header); err != nil {
		return err
	}
	if header.APIVersion == v1beta1.APIVersion {
		return updateBetaImages(pkgPath.ManifestFile, contents, definitionImageResults)
	}
	return modifyManifest(packagePath, func(zarfPackage v1alpha1.ZarfPackage, astFile *ast.File, manifestPath string) (bool, error) {
		if !imageUpdateNeeded(zarfPackage, definitionImageResults) {
			l.Info("no update needed, images are already up to date", "path", manifestPath)
			return false, nil
		}
		if err := createImageUpdate(zarfPackage, definitionImageResults, astFile); err != nil {
			return false, fmt.Errorf("failed to create update: %w", err)
		}
		l.Info("successfully updated images", "path", manifestPath)
		return true, nil
	})
}

func updateBetaImages(manifestPath string, contents []byte, results []DefinitionImageResult) error {
	var header struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(contents, &header); err != nil {
		return err
	}
	var components []v1beta1.Component
	componentConfig := header.Kind == string(v1beta1.ZarfComponentConfig)
	if componentConfig {
		var config v1beta1.ComponentConfig
		if err := yaml.Unmarshal(contents, &config); err != nil {
			return err
		}
		components = []v1beta1.Component{{Name: config.Metadata.Name, ComponentSpec: config.Component}}
	} else {
		var pkg v1beta1.Package
		if err := yaml.Unmarshal(contents, &pkg); err != nil {
			return err
		}
		components = pkg.Components
	}
	astFile, err := parser.ParseBytes(contents, parser.ParseComments)
	if err != nil {
		return err
	}
	type componentKey struct {
		name, architecture, flavor string
	}
	byComponent := make(map[componentKey]DefinitionImageResult, len(results))
	for _, result := range results {
		key := componentKey{result.ComponentName, result.Target.Architecture, result.Target.Flavor}
		byComponent[key] = result
	}
	changed := false
	for index, component := range components {
		key := componentKey{component.Name, component.Selector.Architecture, component.Selector.Flavor}
		result, found := byComponent[key]
		if !found {
			continue
		}
		archives := map[string]struct{}{}
		for _, archive := range result.ImageArchives {
			for _, image := range archive.Images {
				archives[image] = struct{}{}
			}
		}
		existing := make(map[string]v1beta1.Image, len(component.Images))
		for _, image := range component.Images {
			existing[image.Name] = image
		}
		newImages := []v1beta1.Image{}
		seen := map[string]struct{}{}
		for _, name := range slices.Concat(result.Matches, result.PotentialMatches, result.CosignArtifacts) {
			if _, archived := archives[name]; archived {
				continue
			}
			if _, duplicate := seen[name]; duplicate {
				continue
			}
			seen[name] = struct{}{}
			image, found := existing[name]
			if !found {
				image = v1beta1.Image{Name: name}
			}
			newImages = append(newImages, image)
		}
		imagesEqual := slices.EqualFunc(component.Images, newImages, func(a, b v1beta1.Image) bool { return a == b })
		archivesEqual := slices.EqualFunc(component.ImageArchives, result.ImageArchives, func(a v1beta1.ImageArchive, b api.ImageArchive) bool {
			return a.Path == b.Path && slices.Equal(a.Images, b.Images)
		})
		if imagesEqual && archivesEqual {
			continue
		}
		patch := map[string]any{}
		if !imagesEqual {
			patch["images"] = newImages
		}
		if !archivesEqual {
			patch["imageArchives"] = result.ImageArchives
		}
		pathString := fmt.Sprintf("$.components[%d]", index)
		if componentConfig {
			pathString = "$.component"
		}
		node, err := yaml.ValueToNode(patch, yaml.IndentSequence(true))
		if err != nil {
			return err
		}
		path, err := yaml.PathString(pathString)
		if err != nil {
			return err
		}
		if err := path.MergeFromNode(astFile, node); err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return os.WriteFile(manifestPath, []byte(astFile.String()), helpers.ReadAllWriteUser)
}

// modifyManifest loads the zarf.yaml at packagePath, calls fn with the parsed package and AST,
// and writes the result back only if fn signals that a change was made.
func modifyManifest(packagePath string, fn func(v1alpha1.ZarfPackage, *ast.File, string) (bool, error)) error {
	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return fmt.Errorf("unable to access package path %q: %w", packagePath, err)
	}

	b, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", pkgPath.ManifestFile, err)
	}

	var zarfPackage v1alpha1.ZarfPackage
	if err := yaml.Unmarshal(b, &zarfPackage); err != nil {
		return fmt.Errorf("failed to parse zarf.yaml: %w", err)
	}

	astFile, err := parser.ParseBytes(b, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s as AST: %w", pkgPath.ManifestFile, err)
	}

	changed, err := fn(zarfPackage, astFile, pkgPath.ManifestFile)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	if err := os.WriteFile(pkgPath.ManifestFile, []byte(astFile.String()), helpers.ReadAllWriteUser); err != nil {
		return fmt.Errorf("failed to write updated %s: %w", pkgPath.ManifestFile, err)
	}
	return nil
}

func createSchemaUpdate(zarfPackage v1alpha1.ZarfPackage, schemaFilename string, astFile *ast.File) error {
	// If values.files exists we must merge only schema into the existing values map to
	// preserve the files list. Otherwise, create the whole values mapping from scratch.
	var pathStr string
	var patchValue any
	if zarfPackage.Values.Files != nil {
		pathStr = "$.values"
		patchValue = map[string]any{"schema": schemaFilename}
	} else {
		pathStr = "$"
		patchValue = map[string]any{"values": map[string]any{"schema": schemaFilename}}
	}

	patchNode, err := yaml.ValueToNode(patchValue)
	if err != nil {
		return fmt.Errorf("failed to create YAML node for schema: %w", err)
	}

	p, err := yaml.PathString(pathStr)
	if err != nil {
		return fmt.Errorf("failed to create YAML path: %w", err)
	}

	if err := p.MergeFromNode(astFile, patchNode); err != nil {
		return fmt.Errorf("failed to merge schema path: %w", err)
	}

	return nil
}

func createImageUpdate(zarfPackage v1alpha1.ZarfPackage, definitionImageResults []DefinitionImageResult, astFile *ast.File) error {
	// Note: yamlpath support of goccy/go-yaml only has index-based lookup
	componentToIndex := make(map[string]int, len(zarfPackage.Components))
	for i, component := range zarfPackage.Components {
		componentToIndex[component.Name] = i
	}

	for _, result := range definitionImageResults {
		if len(result.Matches)+len(result.PotentialMatches)+len(result.CosignArtifacts)+len(result.ImageArchives) == 0 {
			continue
		}

		componentIndex, exists := componentToIndex[result.ComponentName]
		if !exists {
			continue
		}

		combined := slices.Concat(result.Matches, result.PotentialMatches, result.CosignArtifacts)

		patch := make(map[string]any)

		if len(combined) > 0 {
			patch["images"] = combined
		}

		if len(result.ImageArchives) > 0 {
			patch["imageArchives"] = result.ImageArchives
		}

		if err := patchComponent(patch, result.ComponentName, componentIndex, astFile); err != nil {
			return err
		}
	}
	return nil
}

func patchComponent(patch map[string]any, component string, componentIndex int, astFile *ast.File) error {
	componentNode, err := yaml.ValueToNode(patch, yaml.IndentSequence(true))
	if err != nil {
		return fmt.Errorf("failed to create YAML node for component %s: %w", component, err)
	}

	path, err := yaml.PathString(fmt.Sprintf("$.components[%d]", componentIndex))
	if err != nil {
		return fmt.Errorf("failed to create YAML path for component %s: %w", component, err)
	}

	if err := path.MergeFromNode(astFile, componentNode); err != nil {
		return fmt.Errorf("failed to merge images for component %s: %w", component, err)
	}

	return nil
}

func imageUpdateNeeded(zarfPackage v1alpha1.ZarfPackage, definitionImageResults []DefinitionImageResult) bool {
	definitionImageResultsByComponent := make(map[string]DefinitionImageResult, len(definitionImageResults))
	for _, d := range definitionImageResults {
		definitionImageResultsByComponent[d.ComponentName] = d
	}

	for _, component := range zarfPackage.Components {
		result := definitionImageResultsByComponent[component.Name]

		// Collect archive-scanned images for this component
		archiveScannedImages := make(map[string]struct{})
		for _, ia := range result.ImageArchives {
			for _, img := range ia.Images {
				archiveScannedImages[img] = struct{}{}
			}
		}

		// Check archive images: package definition vs archive scan
		componentArchiveImages := make(map[string]struct{})
		for _, archive := range component.ImageArchives {
			for _, img := range archive.Images {
				componentArchiveImages[img] = struct{}{}
			}
		}
		if !maps.Equal(componentArchiveImages, archiveScannedImages) {
			return true
		}

		// Check regular images: package definition vs image scan
		// Scanned images that also appear in archives are excluded (they're accounted for above)
		scannedImages := make(map[string]struct{})
		for _, img := range slices.Concat(result.Matches, result.PotentialMatches, result.CosignArtifacts) {
			if _, inArchive := archiveScannedImages[img]; !inArchive {
				scannedImages[img] = struct{}{}
			}
		}
		componentImages := make(map[string]struct{}, len(component.Images))
		for _, img := range component.Images {
			componentImages[img] = struct{}{}
		}
		if !maps.Equal(componentImages, scannedImages) {
			return true
		}
	}

	return false
}
