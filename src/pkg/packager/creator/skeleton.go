// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// verify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

// SkeletonCreator provides methods for creating skeleton Zarf packages.
type SkeletonCreator struct {
	createOpts  types.ZarfCreateOptions
	publishOpts types.ZarfPublishOptions
}

// NewSkeletonCreator returns a new SkeletonCreator.
func NewSkeletonCreator(createOpts types.ZarfCreateOptions, publishOpts types.ZarfPublishOptions) *SkeletonCreator {
	return &SkeletonCreator{createOpts, publishOpts}
}

// LoadPackageDefinition loads and configure a zarf.yaml file when creating and publishing a skeleton package.
func (sc *SkeletonCreator) LoadPackageDefinition(ctx context.Context, src *layout.PackagePaths) (pkg v1alpha1.ZarfPackage, warnings []string, err error) {
	l := logger.From(ctx)
	pkg, warnings, err = src.ReadZarfYAML()
	if err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	pkg.Metadata.Architecture = config.GetArch()

	// Compose components into a single zarf.yaml file
	pkg, composeWarnings, err := ComposeComponents(ctx, pkg, sc.createOpts.Flavor)
	if err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	pkg.Metadata.Architecture = zoci.SkeletonArch

	warnings = append(warnings, composeWarnings...)

	for _, warning := range warnings {
		l.Warn(warning)
	}

	if err := Validate(pkg, sc.createOpts.BaseDir, sc.createOpts.SetVariables); err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	return pkg, warnings, nil
}

// Assemble updates all components of the loaded Zarf package with necessary modifications for package assembly.
//
// It processes each component to ensure correct structure and resource locations.
func (sc *SkeletonCreator) Assemble(ctx context.Context, dst *layout.PackagePaths, components []v1alpha1.ZarfComponent, _ string) error {
	for _, component := range components {
		c, err := sc.addComponent(ctx, component, dst)
		if err != nil {
			return err
		}
		components = append(components, *c)
	}

	return nil
}

// Output does the following:
//
// - archives components
//
// - generates checksums for all package files
//
// - writes the loaded zarf.yaml to disk
//
// - signs the package
func (sc *SkeletonCreator) Output(ctx context.Context, dst *layout.PackagePaths, pkg *v1alpha1.ZarfPackage) (err error) {
	for _, component := range pkg.Components {
		if err := dst.Components.Archive(ctx, component, false); err != nil {
			return err
		}
	}

	// Calculate all the checksums
	pkg.Metadata.AggregateChecksum, err = dst.GenerateChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for the package: %w", err)
	}

	if err := recordPackageMetadata(pkg, sc.createOpts); err != nil {
		return err
	}

	if err := utils.WriteYaml(dst.ZarfYAML, pkg, helpers.ReadUser); err != nil {
		return fmt.Errorf("unable to write zarf.yaml: %w", err)
	}

	return dst.SignPackage(sc.publishOpts.SigningKeyPath, sc.publishOpts.SigningKeyPassword, !config.CommonOptions.Confirm)
}

func (sc *SkeletonCreator) addComponent(ctx context.Context, component v1alpha1.ZarfComponent, dst *layout.PackagePaths) (updatedComponent *v1alpha1.ZarfComponent, err error) {
	l := logger.From(ctx)
	logger.From(ctx).Info("processing component", "name", component.Name)

	updatedComponent = &component

	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return nil, err
	}

	if component.DeprecatedCosignKeyPath != "" {
		dst := filepath.Join(componentPaths.Base, "cosign.pub")
		err := helpers.CreatePathAndCopy(component.DeprecatedCosignKeyPath, dst)
		if err != nil {
			return nil, err
		}
		updatedComponent.DeprecatedCosignKeyPath = "cosign.pub"
	}

	// TODO: (@WSTARR) Shim the skeleton component's create action dirs to be empty. This prevents actions from failing by cd'ing into directories that will be flattened.
	updatedComponent.Actions.OnCreate.Defaults.Dir = ""

	resetActions := func(actions []v1alpha1.ZarfComponentAction) []v1alpha1.ZarfComponentAction {
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

			err := helpers.CreatePathAndCopy(chart.LocalPath, dst)
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

			if err := helpers.CreatePathAndCopy(path, filepath.Join(componentPaths.Base, rel)); err != nil {
				return nil, fmt.Errorf("unable to copy chart values file %s: %w", path, err)
			}
		}
	}

	for filesIdx, file := range component.Files {
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
			if err := helpers.CreatePathAndCopy(file.Source, dst); err != nil {
				return nil, fmt.Errorf("unable to copy file %s: %w", file.Source, err)
			}
		}

		// Change the source to the new relative source directory (any remote files will have been skipped above)
		updatedComponent.Files[filesIdx].Source = rel

		// Remove the extractPath from a skeleton since it will already extract it
		updatedComponent.Files[filesIdx].ExtractPath = ""

		// Abort packaging on invalid shasum (if one is specified).
		if file.Shasum != "" {
			if err := helpers.SHAsMatch(dst, file.Shasum); err != nil {
				return nil, err
			}
		}

		if file.Executable || helpers.IsDir(dst) {
			_ = os.Chmod(dst, helpers.ReadWriteExecuteUser)
		} else {
			_ = os.Chmod(dst, helpers.ReadWriteUser)
		}
	}

	if len(component.DataInjections) > 0 {
		for dataIdx, data := range component.DataInjections {
			l.Debug("copying data injection", "source", data.Source, "target", data.Target.Path)

			rel := filepath.Join(layout.DataInjectionsDir, strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
			dst := filepath.Join(componentPaths.Base, rel)

			if err := helpers.CreatePathAndCopy(data.Source, dst); err != nil {
				return nil, fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
			}

			updatedComponent.DataInjections[dataIdx].Source = rel
		}

	}

	if len(component.Manifests) > 0 {
		// Get the proper count of total manifests to add.
		manifestCount := 0

		for _, manifest := range component.Manifests {
			manifestCount += len(manifest.Files)
			manifestCount += len(manifest.Kustomizations)
		}

		// Iterate over all manifests.
		for manifestIdx, manifest := range component.Manifests {
			for fileIdx, path := range manifest.Files {
				rel := filepath.Join(layout.ManifestsDir, fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
				dst := filepath.Join(componentPaths.Base, rel)

				// Copy manifests without any processing.
				l.Debug("copying manifest", "path", path)

				if err := helpers.CreatePathAndCopy(path, dst); err != nil {
					return nil, fmt.Errorf("unable to copy manifest %s: %w", path, err)
				}

				updatedComponent.Manifests[manifestIdx].Files[fileIdx] = rel
			}

			for kustomizeIdx, path := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package.
				l.Debug("building kustomization", "path", path)

				kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
				rel := filepath.Join(layout.ManifestsDir, kname)
				dst := filepath.Join(componentPaths.Base, rel)

				if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
					return nil, fmt.Errorf("unable to build kustomization %s: %w", path, err)
				}
			}

			// remove kustomizations
			updatedComponent.Manifests[manifestIdx].Kustomizations = nil
		}
	}

	return updatedComponent, nil
}
