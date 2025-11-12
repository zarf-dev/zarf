// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// UpdateImages updates the images field for components in a zarf.yaml
func UpdateImages(ctx context.Context, baseDir string, imagesScans []ComponentImageScan) error {
	l := logger.From(ctx)
	packageConfigFile := filepath.Join(baseDir, layout.ZarfYAML)

	packageConfigBytes, err := os.ReadFile(packageConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", packageConfigFile, err)
	}

	zarfPackage := v1alpha1.ZarfPackage{}
	if err := yaml.Unmarshal(packageConfigBytes, &zarfPackage); err != nil {
		return fmt.Errorf("failed to parse zarf.yaml: %w", err)
	}

	if !updateNeeded(zarfPackage, imagesScans) {
		l.Info("no update needed, images are already up to date", "path", packageConfigFile)
		return nil
	}

	astFile, err := parser.ParseBytes(packageConfigBytes, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s as AST: %w", packageConfigFile, err)
	}

	updatedZarfYaml, err := createUpdate(zarfPackage, imagesScans, astFile)
	if err != nil {
		return fmt.Errorf("failed to create update: %w", err)
	}

	if err := os.WriteFile(packageConfigFile, []byte(updatedZarfYaml), helpers.ReadAllWriteUser); err != nil {
		return fmt.Errorf("failed to write updated %s: %w", packageConfigFile, err)
	}

	l.Info("successfully updated images", "path", packageConfigFile)
	return nil
}

func createUpdate(zarfPackage v1alpha1.ZarfPackage, imagesScans []ComponentImageScan, astFile *ast.File) (string, error) {
	// Note: yamlpath support of goccy/go-yaml only has index-based lookup
	componentToIndex := make(map[string]int, len(zarfPackage.Components))
	for i, component := range zarfPackage.Components {
		componentToIndex[component.Name] = i
	}

	for _, scan := range imagesScans {
		if len(scan.Matches)+len(scan.PotentialMatches)+len(scan.CosignArtifacts) == 0 {
			continue
		}

		componentIndex, exists := componentToIndex[scan.ComponentName]
		if !exists {
			continue
		}

		combined := slices.Concat(scan.Matches, scan.PotentialMatches, scan.CosignArtifacts)

		componentMerge := map[string]any{
			"images": combined,
		}
		componentNode, err := yaml.ValueToNode(componentMerge, yaml.IndentSequence(true))
		if err != nil {
			return "", fmt.Errorf("failed to create YAML node for component %s: %w", scan.ComponentName, err)
		}

		path, err := yaml.PathString(fmt.Sprintf("$.components[%d]", componentIndex))
		if err != nil {
			return "", fmt.Errorf("failed to create YAML path for component %s: %w", scan.ComponentName, err)
		}

		if err := path.MergeFromNode(astFile, componentNode); err != nil {
			return "", fmt.Errorf("failed to merge images for component %s: %w", scan.ComponentName, err)
		}
	}

	return astFile.String(), nil
}

func updateNeeded(zarfPackage v1alpha1.ZarfPackage, imageScans []ComponentImageScan) bool {
	scanMap := make(map[string]map[string]struct{}, len(imageScans))

	for _, scan := range imageScans {
		combined := slices.Concat(scan.Matches, scan.PotentialMatches, scan.CosignArtifacts)
		imageSet := make(map[string]struct{}, len(combined))
		for _, img := range combined {
			imageSet[img] = struct{}{}
		}
		scanMap[scan.ComponentName] = imageSet
	}

	for _, component := range zarfPackage.Components {
		imageSet, found := scanMap[component.Name]
		if !found {
			return true
		}

		for _, img := range component.Images {
			if _, found := imageSet[img]; !found {
				return true
			}
		}
	}

	componentMap := make(map[string]map[string]struct{}, len(zarfPackage.Components))
	for _, component := range zarfPackage.Components {
		imageSet := make(map[string]struct{}, len(component.Images))
		for _, img := range component.Images {
			imageSet[img] = struct{}{}
		}
		componentMap[component.Name] = imageSet
	}

	for _, scan := range imageScans {
		componentImages, found := componentMap[scan.ComponentName]
		if !found {
			return true
		}

		combined := slices.Concat(scan.Matches, scan.PotentialMatches, scan.CosignArtifacts)
		for _, img := range combined {
			if _, found := componentImages[img]; !found {
				return true
			}
		}
	}

	return false
}
