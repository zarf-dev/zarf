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
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// UpdateImages updates the images field for components in a zarf.yaml
func UpdateImages(ctx context.Context, packagePath string, imagesScans []ComponentImageScan, archiveImageScans []ImageArchivesScan) error {
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

func createUpdate(zarfPackage v1alpha1.ZarfPackage, imagesScans []ComponentImageScan, archiveImagesScans []ImageArchivesScan, astFile *ast.File) (string, error) {
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

		patch := map[string]any{
			"images": combined,
		}

		err := patchComponent(patch, scan.ComponentName, componentIndex, astFile)

		if err != nil {
			return "", err
		}
	}

	for _, scan := range archiveImagesScans {
		componentIndex, exists := componentToIndex[scan.ComponentName]
		if !exists {
			continue
		}

		patch := map[string]any{
			"imageArchives": scan.ImageArchives,
		}

		err := patchComponent(patch, scan.ComponentName, componentIndex, astFile)

		if err != nil {
			return "", err
		}
	}

	return astFile.String(), nil
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

func updateNeeded(zarfPackage v1alpha1.ZarfPackage, imageScans []ComponentImageScan, archiveImagesScans []ImageArchivesScan) bool {

	imageScansByComponent := make(map[string]ComponentImageScan, len(imageScans))
	for _, s := range imageScans {
		imageScansByComponent[s.ComponentName] = s
	}
	archiveScansByComponent := make(map[string]ImageArchivesScan, len(archiveImagesScans))
	for _, s := range archiveImagesScans {
		archiveScansByComponent[s.ComponentName] = s
	}

	for _, component := range zarfPackage.Components {
		scan := imageScansByComponent[component.Name]
		archiveScan := archiveScansByComponent[component.Name]

		// Collect archive-scanned images for this component
		archiveScannedImages := make(map[string]struct{})
		for _, ia := range archiveScan.ImageArchives {
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
		for _, img := range slices.Concat(scan.Matches, scan.PotentialMatches, scan.CosignArtifacts) {
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
