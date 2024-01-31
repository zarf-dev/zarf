// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

var (
	// veryify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

// SkeletonCreator provides methods for creating skeleton Zarf packages.
type SkeletonCreator struct {
	createOpts types.ZarfCreateOptions
}

// LoadPackageDefinition loads and configure a zarf.yaml file during package create.
func (sc *SkeletonCreator) LoadPackageDefinition(dst *layout.PackagePaths) (loadedPkg *types.ZarfPackage, warnings []string, err error) {
	var pkg types.ZarfPackage

	if err := utils.ReadYaml(layout.ZarfYAML, &pkg); err != nil {
		return nil, nil, fmt.Errorf("unable to read the zarf.yaml file: %w", err)
	}

	configuredPkg, err := setPackageMetadata(pkg, sc.createOpts)
	if err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	composedPkg, composeWarnings, err := ComposeComponents(configuredPkg, sc.createOpts.Flavor)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, composeWarnings...)

	extendedPkg, err := sc.processExtensions(composedPkg, dst)
	if err != nil {
		return nil, nil, err
	}

	loadedPkg = extendedPkg

	return loadedPkg, warnings, nil
}

// TODO: print warnings somewhere else in the skeleton create flow.
func (sc *SkeletonCreator) Assemble(loadedPkg *types.ZarfPackage, dst *layout.PackagePaths) error {
	var updatedComponents []types.ZarfComponent

	for _, component := range loadedPkg.Components {
		c, err := sc.addComponent(component, dst)
		if err != nil {
			return err
		}
		updatedComponents = append(updatedComponents, *c)
	}

	loadedPkg.Components = updatedComponents

	return nil
}

func (sc *SkeletonCreator) Output(loadedPkg *types.ZarfPackage, dst *layout.PackagePaths) error {
	for _, component := range loadedPkg.Components {
		if err := dst.Components.Archive(component, false); err != nil {
			return err
		}
	}

	checksumChecksum, err := generateChecksums(dst)
	if err != nil {
		return fmt.Errorf("unable to generate checksums for skeleton package: %w", err)
	}
	loadedPkg.Metadata.AggregateChecksum = checksumChecksum

	return utils.WriteYaml(dst.ZarfYAML, loadedPkg, 0400)
}

func (sc *SkeletonCreator) processExtensions(pkg *types.ZarfPackage, layout *layout.PackagePaths) (extendedPkg *types.ZarfPackage, err error) {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range pkg.Components {
		componentPaths, err := layout.Components.Create(c)
		if err != nil {
			return nil, err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Skeletonize(componentPaths, c); err != nil {
				return nil, fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		components = append(components, c)
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	pkg.Components = components

	extendedPkg = pkg

	return extendedPkg, nil
}

func (sc *SkeletonCreator) addComponent(component types.ZarfComponent, dst *layout.PackagePaths) (updatedComponent *types.ZarfComponent, err error) {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	updatedComponent = &component

	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return nil, err
	}

	if component.DeprecatedCosignKeyPath != "" {
		dst := filepath.Join(componentPaths.Base, "cosign.pub")
		err := utils.CreatePathAndCopy(component.DeprecatedCosignKeyPath, dst)
		if err != nil {
			return nil, err
		}
		updatedComponent.DeprecatedCosignKeyPath = "cosign.pub"
	}

	// TODO: (@WSTARR) Shim the skeleton component's create action dirs to be empty. This prevents actions from failing by cd'ing into directories that will be flattened.
	updatedComponent.Actions.OnCreate.Defaults.Dir = ""

	resetActions := func(actions []types.ZarfComponentAction) []types.ZarfComponentAction {
		for idx := range actions {
			actions[idx].Dir = nil
		}
		return actions
	}

	updatedComponent.Actions.OnCreate.Before = resetActions(component.Actions.OnCreate.Before)
	updatedComponent.Actions.OnCreate.After = resetActions(component.Actions.OnCreate.After)
	updatedComponent.Actions.OnCreate.OnSuccess = resetActions(component.Actions.OnCreate.OnSuccess)
	updatedComponent.Actions.OnCreate.OnFailure = resetActions(component.Actions.OnCreate.OnFailure)

	// If any helm charts are defined, process them.
	for chartIdx, chart := range component.Charts {

		if chart.LocalPath != "" {
			rel := filepath.Join(layout.ChartsDir, fmt.Sprintf("%s-%d", chart.Name, chartIdx))
			dst := filepath.Join(componentPaths.Base, rel)

			err := utils.CreatePathAndCopy(chart.LocalPath, dst)
			if err != nil {
				return nil, err
			}

			updatedComponent.Charts[chartIdx].LocalPath = rel
		}

		for valuesIdx, path := range chart.ValuesFiles {
			if helpers.IsURL(path) {
				continue
			}

			rel := fmt.Sprintf("%s-%d", helm.StandardName(layout.ValuesDir, chart), valuesIdx)
			updatedComponent.Charts[chartIdx].ValuesFiles[valuesIdx] = rel

			if err := utils.CreatePathAndCopy(path, filepath.Join(componentPaths.Base, rel)); err != nil {
				return nil, fmt.Errorf("unable to copy chart values file %s: %w", path, err)
			}
		}
	}

	for filesIdx, file := range component.Files {
		message.Debugf("Loading %#v", file)

		if helpers.IsURL(file.Source) {
			continue
		}

		rel := filepath.Join(layout.FilesDir, strconv.Itoa(filesIdx), filepath.Base(file.Target))
		dst := filepath.Join(componentPaths.Base, rel)
		destinationDir := filepath.Dir(dst)

		if file.ExtractPath != "" {
			if err := archiver.Extract(file.Source, file.ExtractPath, destinationDir); err != nil {
				return nil, fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, file.Source, err.Error())
			}

			// Make sure dst reflects the actual file or directory.
			updatedExtractedFileOrDir := filepath.Join(destinationDir, file.ExtractPath)
			if updatedExtractedFileOrDir != dst {
				if err := os.Rename(updatedExtractedFileOrDir, dst); err != nil {
					return nil, fmt.Errorf(lang.ErrWritingFile, dst, err)
				}
			}
		} else {
			if err := utils.CreatePathAndCopy(file.Source, dst); err != nil {
				return nil, fmt.Errorf("unable to copy file %s: %w", file.Source, err)
			}
		}

		// Change the source to the new relative source directory (any remote files will have been skipped above)
		updatedComponent.Files[filesIdx].Source = rel

		// Remove the extractPath from a skeleton since it will already extract it
		updatedComponent.Files[filesIdx].ExtractPath = ""

		// Abort packaging on invalid shasum (if one is specified).
		if file.Shasum != "" {
			if err := utils.SHAsMatch(dst, file.Shasum); err != nil {
				return nil, err
			}
		}

		if file.Executable || utils.IsDir(dst) {
			_ = os.Chmod(dst, 0700)
		} else {
			_ = os.Chmod(dst, 0600)
		}
	}

	if len(component.DataInjections) > 0 {
		spinner := message.NewProgressSpinner("Loading data injections")
		defer spinner.Stop()

		for dataIdx, data := range component.DataInjections {
			spinner.Updatef("Copying data injection %s for %s", data.Target.Path, data.Target.Selector)

			rel := filepath.Join(layout.DataInjectionsDir, strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
			dst := filepath.Join(componentPaths.Base, rel)

			if err := utils.CreatePathAndCopy(data.Source, dst); err != nil {
				return nil, fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
			}

			updatedComponent.DataInjections[dataIdx].Source = rel
		}

		spinner.Success()
	}

	if len(component.Manifests) > 0 {
		// Get the proper count of total manifests to add.
		manifestCount := 0

		for _, manifest := range component.Manifests {
			manifestCount += len(manifest.Files)
			manifestCount += len(manifest.Kustomizations)
		}

		spinner := message.NewProgressSpinner("Loading %d K8s manifests", manifestCount)
		defer spinner.Stop()

		// Iterate over all manifests.
		for manifestIdx, manifest := range component.Manifests {
			for fileIdx, path := range manifest.Files {
				rel := filepath.Join(layout.ManifestsDir, fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
				dst := filepath.Join(componentPaths.Base, rel)

				// Copy manifests without any processing.
				spinner.Updatef("Copying manifest %s", path)

				if err := utils.CreatePathAndCopy(path, dst); err != nil {
					return nil, fmt.Errorf("unable to copy manifest %s: %w", path, err)
				}

				updatedComponent.Manifests[manifestIdx].Files[fileIdx] = rel
			}

			for kustomizeIdx, path := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package.
				spinner.Updatef("Building kustomization for %s", path)

				kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
				rel := filepath.Join(layout.ManifestsDir, kname)
				dst := filepath.Join(componentPaths.Base, rel)

				if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
					return nil, fmt.Errorf("unable to build kustomization %s: %w", path, err)
				}

				updatedComponent.Manifests[manifestIdx].Files = append(updatedComponent.Manifests[manifestIdx].Files, rel)
			}
			// remove kustomizations
			updatedComponent.Manifests[manifestIdx].Kustomizations = nil
		}

		spinner.Success()
	}

	return updatedComponent, nil
}