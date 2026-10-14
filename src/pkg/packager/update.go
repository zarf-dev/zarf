// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// UpdateSchema updates the values.schema field in a zarf.yaml to point to the given relative schema filename.
func UpdateSchema(ctx context.Context, packagePath string, schemaFilename string) error {
	l := logger.From(ctx)
	return modifyManifest(packagePath, func(astFile *ast.File, manifestPath string) (bool, error) {
		if err := createSchemaUpdate(schemaFilename, astFile); err != nil {
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
	header, err := load.ParseDefinitionHeader(contents)
	if err != nil {
		return err
	}
	if header.APIVersion == v1beta1.APIVersion {
		return updateBetaImages(pkgPath.ManifestFile, contents, header.Kind, definitionImageResults)
	}
	return modifyManifest(packagePath, func(astFile *ast.File, manifestPath string) (bool, error) {
		var zarfPackage v1alpha1.ZarfPackage
		if err := yaml.NodeToValue(astFile.Docs[0].Body, &zarfPackage); err != nil {
			return false, fmt.Errorf("failed to parse zarf.yaml: %w", err)
		}
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

func updateBetaImages(manifestPath string, contents []byte, kind string, results []DefinitionImageResult) error {
	var components []v1beta1.Component
	componentConfig := kind == string(v1beta1.ZarfComponentConfig)
	switch kind {
	case string(v1beta1.ZarfComponentConfig):
		var config v1beta1.ComponentConfig
		if err := yaml.Unmarshal(contents, &config); err != nil {
			return err
		}
		components = []v1beta1.Component{{Name: config.Metadata.Name, ComponentSpec: config.Component}}
	// TODO, when we add v1beta1 init configs, we'll have to allow that here as well
	case string(v1beta1.ZarfPackageConfig):
		var pkg v1beta1.Package
		if err := yaml.Unmarshal(contents, &pkg); err != nil {
			return err
		}
		components = pkg.Components
	default:
		return fmt.Errorf("invalid kind %q", kind)
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
		key := componentKey{result.ComponentName, result.Selector.Architecture, result.Selector.Flavor}
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
				ref, err := transform.ParseImageRef(image)
				if err != nil {
					return fmt.Errorf("invalid image %q in archive %q: %w", image, archive.Path, err)
				}
				archives[ref.Reference] = struct{}{}
			}
		}
		existing := make(map[string]v1beta1.Image, len(component.Images))
		for _, image := range component.Images {
			ref, err := transform.ParseImageRef(image.Name)
			if err != nil {
				return fmt.Errorf("invalid image %q in component %q: %w", image.Name, component.Name, err)
			}
			existing[ref.Reference] = image
		}
		foundImages := result.Matches
		newImages := []v1beta1.Image{}
		newByRef := make(map[string]v1beta1.Image, len(foundImages))
		for _, match := range foundImages {
			name := match.Image.Name
			ref, err := transform.ParseImageRef(name)
			if err != nil {
				return fmt.Errorf("invalid discovered image %q in component %q: %w", name, component.Name, err)
			}
			if _, archived := archives[ref.Reference]; archived {
				continue
			}
			if _, duplicate := newByRef[ref.Reference]; duplicate {
				continue
			}
			image, found := existing[ref.Reference]
			if !found {
				image = v1beta1.Image{Name: name}
			}
			newByRef[ref.Reference] = image
			newImages = append(newImages, image)
		}
		// An empty scan does not replace authored images.
		imagesUpToDate := len(foundImages) == 0 || (len(component.Images) == len(newImages) && maps.Equal(existing, newByRef))
		// Import resolution appends archives authored here after inherited archives.
		if len(result.ImageArchives) < len(component.ImageArchives) {
			return fmt.Errorf("component %q has fewer scanned archives than authored archives", component.Name)
		}
		authoredArchiveResults := result.ImageArchives[len(result.ImageArchives)-len(component.ImageArchives):]
		for i, archive := range component.ImageArchives {
			if archive.Path != authoredArchiveResults[i].Path {
				return fmt.Errorf("component %q archive %d: expected %q, got %q", component.Name, i, archive.Path, authoredArchiveResults[i].Path)
			}
		}
		archivesEqual := slices.EqualFunc(component.ImageArchives, authoredArchiveResults, func(a v1beta1.ImageArchive, b api.ImageArchive) bool {
			return a.Path == b.Path && slices.Equal(a.Images, b.Images)
		})
		if imagesUpToDate && archivesEqual {
			continue
		}
		patch := map[string]any{}
		if !imagesUpToDate {
			patch["images"] = newImages
		}
		if !archivesEqual {
			patch["imageArchives"] = authoredArchiveResults
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

// modifyManifest loads the definition at packagePath, calls fn with its AST,
// and writes the result back only if fn signals that a change was made.
func modifyManifest(packagePath string, fn func(*ast.File, string) (bool, error)) error {
	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return fmt.Errorf("unable to access package path %q: %w", packagePath, err)
	}

	b, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", pkgPath.ManifestFile, err)
	}

	astFile, err := parser.ParseBytes(b, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s as AST: %w", pkgPath.ManifestFile, err)
	}

	if len(astFile.Docs) == 0 || astFile.Docs[0].Body == nil {
		return fmt.Errorf("definition %s is empty", pkgPath.ManifestFile)
	}

	changed, err := fn(astFile, pkgPath.ManifestFile)
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

func createSchemaUpdate(schemaFilename string, astFile *ast.File) error {
	// Read only values so unrelated fields can use either API version.
	var definition struct {
		Values map[string]any `json:"values"`
	}
	if err := yaml.NodeToValue(astFile.Docs[0].Body, &definition); err != nil {
		return err
	}
	// Merge into an existing values map to preserve files and other metadata.
	var pathStr string
	var patchValue any
	if definition.Values != nil {
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
		if len(result.Matches)+len(result.ImageArchives) == 0 {
			continue
		}

		componentIndex, exists := componentToIndex[result.ComponentName]
		if !exists {
			continue
		}

		combined := imageMatchNames(result.Matches)

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
		for _, match := range result.Matches {
			img := match.Image.Name
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
