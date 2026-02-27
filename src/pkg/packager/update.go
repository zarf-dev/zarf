// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

type ImagesPatch struct {
	Images []string `yaml:"images"`
}

type ImageArchivesPatch struct {
	ImageArchives ImagesPatch `yaml:"imageArchives"`
}

// UpdateImages updates the images field for components in a zarf.yaml
func UpdateImages(ctx context.Context, packagePath string, imagesScans []ComponentImageScan, archiveImageScans []ArchiveImageScan) error {
	l := logger.From(ctx)

	pkgPath, err := layout.ResolvePackagePath(packagePath)
	if err != nil {
		return fmt.Errorf("unable to access package path %q: %w", packagePath, err)
	}

	packageConfigBytes, err := os.ReadFile(pkgPath.ManifestFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", pkgPath.ManifestFile, err)
	}

	zarfPackage := v1alpha1.ZarfPackage{}
	if err := yaml.Unmarshal(packageConfigBytes, &zarfPackage); err != nil {
		return fmt.Errorf("failed to parse zarf.yaml: %w", err)
	}

	if !updateNeeded(zarfPackage, imagesScans, archiveImageScans) {
		l.Info("no update needed, images are already up to date", "path", pkgPath.ManifestFile)
		return nil
	}

	astFile, err := parser.ParseBytes(packageConfigBytes, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s as AST: %w", pkgPath.ManifestFile, err)
	}

	updatedZarfYaml, err := createUpdate(zarfPackage, imagesScans, archiveImageScans, astFile)
	if err != nil {
		return fmt.Errorf("failed to create update: %w", err)
	}

	if err := os.WriteFile(pkgPath.ManifestFile, []byte(updatedZarfYaml), helpers.ReadAllWriteUser); err != nil {
		return fmt.Errorf("failed to write updated %s: %w", pkgPath.ManifestFile, err)
	}

	l.Info("successfully updated images", "path", pkgPath.ManifestFile)
	return nil
}

func createUpdate(zarfPackage v1alpha1.ZarfPackage, imagesScans []ComponentImageScan, archiveImagesScans []ArchiveImageScan, astFile *ast.File) (string, error) {
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

	for _, scan := range archiveImagesScans {
		componentIndex, exists := componentToIndex[scan.ComponentName]
		if !exists {
			continue
		}

		patch := ImageArchivesPatch{
			ImageArchives: ImagesPatch{scan.Images},
		}

		componentNode, err := yaml.ValueToNode(patch, yaml.IndentSequence(true))
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

func updateNeeded(zarfPackage v1alpha1.ZarfPackage, imageScans []ComponentImageScan, archiveImagesScans []ArchiveImageScan) bool {
	scanMap := make(map[string]map[string]struct{}, len(imageScans))
	archiveScanMap := make(map[string]map[string]struct{}, len(archiveImagesScans))

	// Map components to all archive images found in archive scans
	for _, scan := range archiveImagesScans {
		imageSet := make(map[string]struct{}, len(scan.Images))
		for _, img := range scan.Images {
			imageSet[img] = struct{}{}
		}
		archiveScanMap[scan.ComponentName] = imageSet
	}

	// Map comonents to all images found in scan, discounting any images that are included in image archives
	for _, scan := range imageScans {
		combined := slices.Concat(scan.Matches, scan.PotentialMatches, scan.CosignArtifacts)
		imageSet := make(map[string]struct{}, len(combined))
		for _, img := range combined {
			if _, found := archiveScanMap[scan.ComponentName][img]; found {
				continue
			}
			imageSet[img] = struct{}{}
		}
		scanMap[scan.ComponentName] = imageSet
	}

	// Update needed if:
	// 1. A component is found in the package definition that is not included in the scan
	// 2. An image is found in the component images is not included in the scan images
	// 3. An image is found in the component imageArchives is not included in the archive scan images
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

		for _, archive := range component.ImageArchives {
			for _, img := range archive.Images {
				if _, found := archiveScanMap[component.Name][img]; !found {
					return true
				}
			}
		}
	}

	// Map component to component images
	componentMap := make(map[string]map[string]struct{}, len(zarfPackage.Components))
	for _, component := range zarfPackage.Components {
		imageSet := make(map[string]struct{}, len(component.Images))
		for _, img := range component.Images {
			imageSet[img] = struct{}{}
		}
		componentMap[component.Name] = imageSet
	}

	// Update needed if:
	// 1. A component is found in the scan that is not included in the package definition
	// 2. An image is found in the scan is not included in the component images
	for _, scan := range imageScans {
		componentImages, found := componentMap[scan.ComponentName]
		if !found {
			return true
		}

		combined := slices.Concat(scan.Matches, scan.PotentialMatches, scan.CosignArtifacts)
		for _, img := range combined {
			_, foundInComponent := componentImages[img]
			_, foundInArchive := archiveScanMap[scan.ComponentName][img]
			if !foundInComponent && !foundInArchive {
				return true
			}
		}
	}

	// Map image archive components to archvie component images
	componentArchiveMap := make(map[string]map[string]struct{}, len(zarfPackage.Components))
	for _, component := range zarfPackage.Components {
		imageSet := make(map[string]struct{})
		for _, archive := range component.ImageArchives {
			for _, img := range archive.Images {
				imageSet[img] = struct{}{}
			}
		}
		componentArchiveMap[component.Name] = imageSet
	}

	// Update needed if:
	// 1. A component is found in the archive scan that is not included in the package definition
	// 2. An image is found in the archive scan that is not included in the component archive images
	for _, scan := range archiveImagesScans {
		componentArchiveImages, found := componentArchiveMap[scan.ComponentName]
		if !found {
			return true
		}

		for _, img := range scan.Images {
			_, found := componentArchiveImages[img]
			if !found {
				return true
			}
		}
	}

	return false
}
