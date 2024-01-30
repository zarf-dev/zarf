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
	cfg *types.PackagerConfig
}

// LoadPackageDefinition loads and configure a zarf.yaml file during package create.
func (sc *SkeletonCreator) LoadPackageDefinition(pkg types.ZarfPackage, dst *layout.PackagePaths) (loadedPkg types.ZarfPackage, warnings []string, err error) {
	configuredPkg, err := setPackageMetadata(pkg, sc.cfg.CreateOpts)
	if err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	composedPkg, composeWarnings, err := ComposeComponents(configuredPkg, sc.cfg.CreateOpts)
	if err != nil {
		return pkg, nil, err
	}
	warnings = append(warnings, composeWarnings...)

	extendedPkg, err := processExtensions(composedPkg, sc.cfg.CreateOpts, dst)
	if err != nil {
		return pkg, nil, err
	}

	loadedPkg = extendedPkg

	return loadedPkg, warnings, nil
}

// TODO: print warnings somewhere else in the skeleton create flow.
func (sc *SkeletonCreator) Assemble(pkg types.ZarfPackage, dst *layout.PackagePaths) error {
	for idx, component := range pkg.Components {
		if err := sc.addComponent(idx, component, dst); err != nil {
			return err
		}
	}
	return nil
}

func (sc *SkeletonCreator) Output(pkg types.ZarfPackage, dst *layout.PackagePaths) error {
	for _, component := range pkg.Components {
		if err := dst.Components.Archive(component, false); err != nil {
			return err
		}
	}

	checksumChecksum, err := generateChecksums(dst)
	if err != nil {
		return fmt.Errorf("unable to generate checksums for skeleton package: %w", err)
	}
	pkg.Metadata.AggregateChecksum = checksumChecksum

	return utils.WriteYaml(dst.ZarfYAML, pkg, 0400)
}

func (sc *SkeletonCreator) addComponent(index int, component types.ZarfComponent, dst *layout.PackagePaths) error {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return err
	}

	if component.DeprecatedCosignKeyPath != "" {
		dst := filepath.Join(componentPaths.Base, "cosign.pub")
		err := utils.CreatePathAndCopy(component.DeprecatedCosignKeyPath, dst)
		if err != nil {
			return err
		}
		sc.cfg.Pkg.Components[index].DeprecatedCosignKeyPath = "cosign.pub"
	}

	// TODO: (@WSTARR) Shim the skeleton component's create action dirs to be empty. This prevents actions from failing by cd'ing into directories that will be flattened.
	component.Actions.OnCreate.Defaults.Dir = ""

	resetActions := func(actions []types.ZarfComponentAction) []types.ZarfComponentAction {
		for idx := range actions {
			actions[idx].Dir = nil
		}
		return actions
	}

	component.Actions.OnCreate.Before = resetActions(component.Actions.OnCreate.Before)
	component.Actions.OnCreate.After = resetActions(component.Actions.OnCreate.After)
	component.Actions.OnCreate.OnSuccess = resetActions(component.Actions.OnCreate.OnSuccess)
	component.Actions.OnCreate.OnFailure = resetActions(component.Actions.OnCreate.OnFailure)

	// If any helm charts are defined, process them.
	for chartIdx, chart := range component.Charts {

		if chart.LocalPath != "" {
			rel := filepath.Join(layout.ChartsDir, fmt.Sprintf("%s-%d", chart.Name, chartIdx))
			dst := filepath.Join(componentPaths.Base, rel)

			err := utils.CreatePathAndCopy(chart.LocalPath, dst)
			if err != nil {
				return err
			}

			sc.cfg.Pkg.Components[index].Charts[chartIdx].LocalPath = rel
		}

		for valuesIdx, path := range chart.ValuesFiles {
			if helpers.IsURL(path) {
				continue
			}

			rel := fmt.Sprintf("%s-%d", helm.StandardName(layout.ValuesDir, chart), valuesIdx)
			sc.cfg.Pkg.Components[index].Charts[chartIdx].ValuesFiles[valuesIdx] = rel

			if err := utils.CreatePathAndCopy(path, filepath.Join(componentPaths.Base, rel)); err != nil {
				return fmt.Errorf("unable to copy chart values file %s: %w", path, err)
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
				return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, file.Source, err.Error())
			}

			// Make sure dst reflects the actual file or directory.
			updatedExtractedFileOrDir := filepath.Join(destinationDir, file.ExtractPath)
			if updatedExtractedFileOrDir != dst {
				if err := os.Rename(updatedExtractedFileOrDir, dst); err != nil {
					return fmt.Errorf(lang.ErrWritingFile, dst, err)
				}
			}
		} else {
			if err := utils.CreatePathAndCopy(file.Source, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", file.Source, err)
			}
		}

		// Change the source to the new relative source directory (any remote files will have been skipped above)
		sc.cfg.Pkg.Components[index].Files[filesIdx].Source = rel

		// Remove the extractPath from a skeleton since it will already extract it
		sc.cfg.Pkg.Components[index].Files[filesIdx].ExtractPath = ""

		// Abort packaging on invalid shasum (if one is specified).
		if file.Shasum != "" {
			if err := utils.SHAsMatch(dst, file.Shasum); err != nil {
				return err
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
				return fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
			}

			sc.cfg.Pkg.Components[index].DataInjections[dataIdx].Source = rel
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
					return fmt.Errorf("unable to copy manifest %s: %w", path, err)
				}

				sc.cfg.Pkg.Components[index].Manifests[manifestIdx].Files[fileIdx] = rel
			}

			for kustomizeIdx, path := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package.
				spinner.Updatef("Building kustomization for %s", path)

				kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
				rel := filepath.Join(layout.ManifestsDir, kname)
				dst := filepath.Join(componentPaths.Base, rel)

				if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
					return fmt.Errorf("unable to build kustomization %s: %w", path, err)
				}

				sc.cfg.Pkg.Components[index].Manifests[manifestIdx].Files = append(sc.cfg.Pkg.Components[index].Manifests[manifestIdx].Files, rel)
			}
			// remove kustomizations
			sc.cfg.Pkg.Components[index].Manifests[manifestIdx].Kustomizations = nil
		}

		spinner.Success()
	}

	return nil
}
