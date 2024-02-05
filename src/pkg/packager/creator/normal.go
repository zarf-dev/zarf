// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/actions"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

var (
	// veryify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)
)

// PackageCreator provides methods for creating normal (not skeleton) Zarf packages.
type PackageCreator struct {
	createOpts types.ZarfCreateOptions
	cfg        *types.PackagerConfig
}

// LoadPackageDefinition loads and configures a zarf.yaml file during package create.
func (pc *PackageCreator) LoadPackageDefinition(dst *layout.PackagePaths) (loadedPkg *types.ZarfPackage, warnings []string, err error) {
	var pkg types.ZarfPackage

	if err := utils.ReadYaml(layout.ZarfYAML, &pkg); err != nil {
		return nil, nil, fmt.Errorf("unable to read the zarf.yaml file: %w", err)
	}

	configuredPkg, err := setPackageMetadata(pkg, pc.createOpts)
	if err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	composedPkg, composeWarnings, err := ComposeComponents(configuredPkg, pc.createOpts.Flavor)
	if err != nil {
		return nil, nil, err
	}

	warnings = append(warnings, composeWarnings...)

	// After components are composed, template the active package.
	templatedPkg, templateWarnings, err := FillActiveTemplate(composedPkg, pc.createOpts.SetVariables)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to fill values in template: %w", err)
	}

	warnings = append(warnings, templateWarnings...)

	// After templates are filled process any create extensions
	extendedPkg, err := pc.processExtensions(templatedPkg, dst)
	if err != nil {
		return nil, nil, err
	}

	// If we are creating a differential package, remove duplicate images and repos.
	if extendedPkg.Build.Differential {
		loadedDiffData, err := loadDifferentialData(&pc.createOpts.DifferentialData)
		if err != nil {
			return nil, nil, err
		}

		versionsMatch := loadedDiffData.DifferentialPackageVersion == extendedPkg.Metadata.Version
		if versionsMatch {
			return nil, nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}

		noVersionSet := loadedDiffData.DifferentialPackageVersion == "" || extendedPkg.Metadata.Version == ""
		if noVersionSet {
			return nil, nil, errors.New(lang.PkgCreateErrDifferentialNoVersion)
		}

		diffPkg, err := removeCopiesFromDifferentialPackage(extendedPkg, loadedDiffData)
		if err != nil {
			return nil, nil, err
		}
		return diffPkg, warnings, nil
	}

	return extendedPkg, warnings, nil
}

// Assemble assembles all of the package assets into Zarf's tmp directory layout.
func (pc *PackageCreator) Assemble(dst *layout.PackagePaths, loadedPkg *types.ZarfPackage) error {
	var imageList []transform.Image

	skipSBOMFlagUsed := pc.createOpts.SkipSBOM
	componentSBOMs := map[string]*layout.ComponentSBOM{}

	for _, component := range loadedPkg.Components {
		onCreate := component.Actions.OnCreate

		onFailure := func() {
			if err := actions.Run(pc.cfg, onCreate.Defaults, onCreate.OnFailure, nil); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}

		if err := pc.addComponent(component, dst); err != nil {
			onFailure()
			return fmt.Errorf("unable to add component %q: %w", component.Name, err)
		}

		if err := actions.Run(pc.cfg, onCreate.Defaults, onCreate.OnSuccess, nil); err != nil {
			onFailure()
			return fmt.Errorf("unable to run component success action: %w", err)
		}

		if !skipSBOMFlagUsed {
			componentSBOM, err := pc.getFilesToSBOM(component, dst)
			if err != nil {
				return fmt.Errorf("unable to create component SBOM: %w", err)
			}
			if componentSBOM != nil && len(componentSBOM.Files) > 0 {
				componentSBOMs[component.Name] = componentSBOM
			}
		}

		// Combine all component images into a single entry for efficient layer reuse.
		for _, src := range component.Images {
			refInfo, err := transform.ParseImageRef(src)
			if err != nil {
				return fmt.Errorf("failed to create ref for image %s: %w", src, err)
			}
			imageList = append(imageList, refInfo)
		}
	}

	imageList = helpers.Unique(imageList)
	var sbomImageList []transform.Image

	// Images are handled separately from other component assets.
	if len(imageList) > 0 {
		message.HeaderInfof("📦 PACKAGE IMAGES")

		dst.AddImages()

		var pulled []images.ImgInfo
		var err error

		doPull := func() error {
			imgConfig := images.ImageConfig{
				ImagesPath:        dst.Images.Base,
				ImageList:         imageList,
				Insecure:          config.CommonOptions.Insecure,
				Architectures:     []string{loadedPkg.Metadata.Architecture, loadedPkg.Build.Architecture},
				RegistryOverrides: pc.createOpts.RegistryOverrides,
			}

			pulled, err = imgConfig.PullAll()
			return err
		}

		if err := helpers.Retry(doPull, 3, 5*time.Second, message.Warnf); err != nil {
			return fmt.Errorf("unable to pull images after 3 attempts: %w", err)
		}

		for _, imgInfo := range pulled {
			if err := dst.Images.AddV1Image(imgInfo.Img); err != nil {
				return err
			}
			if imgInfo.HasImageLayers {
				sbomImageList = append(sbomImageList, imgInfo.RefInfo)
			}
		}
	}

	// Ignore SBOM creation if the flag is set.
	if skipSBOMFlagUsed {
		message.Debug("Skipping image SBOM processing per --skip-sbom flag")
	} else {
		dst.AddSBOMs()
		if err := sbom.Catalog(componentSBOMs, sbomImageList, dst); err != nil {
			return fmt.Errorf("unable to create an SBOM catalog for the package: %w", err)
		}
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
//
// - writes the Zarf package as a tarball to a local directory,
// or an OCI registry based on the --output flag
func (pc *PackageCreator) Output(dst *layout.PackagePaths, loadedPkg *types.ZarfPackage) error {
	// Process the component directories into compressed tarballs
	// NOTE: This is purposefully being done after the SBOM cataloging
	for _, component := range loadedPkg.Components {
		// Make the component a tar archive
		if err := dst.Components.Archive(component, true); err != nil {
			return fmt.Errorf("unable to archive component: %s", err.Error())
		}
	}

	// Calculate all the checksums
	checksumChecksum, err := dst.GenerateChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for the package: %w", err)
	}
	loadedPkg.Metadata.AggregateChecksum = checksumChecksum

	// Record the migrations that will be ran on the package.
	loadedPkg.Build.Migrations = []string{
		deprecated.ScriptsToActionsMigrated,
		deprecated.PluralizeSetVariable,
	}

	// Save the transformed config.
	if err := utils.WriteYaml(dst.ZarfYAML, loadedPkg, 0400); err != nil {
		return fmt.Errorf("unable to write zarf.yaml: %w", err)
	}

	// Sign the package if a key has been provided
	if pc.createOpts.SigningKeyPath != "" {
		if err := dst.SignPackage(pc.createOpts.SigningKeyPath, pc.createOpts.SigningKeyPassword); err != nil {
			return err
		}
	}

	// Create a remote ref + client for the package (if output is OCI)
	// then publish the package to the remote.
	if helpers.IsOCIURL(pc.createOpts.Output) {
		ref, err := oci.ReferenceFromMetadata(pc.createOpts.Output, &loadedPkg.Metadata, &loadedPkg.Build)
		if err != nil {
			return err
		}
		remote, err := oci.NewOrasRemote(ref, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			return err
		}

		err = remote.PublishPackage(loadedPkg, dst, config.CommonOptions.OCIConcurrency)
		if err != nil {
			return fmt.Errorf("unable to publish package: %w", err)
		}
		message.HorizontalRule()
		flags := ""
		if config.CommonOptions.Insecure {
			flags = "--insecure"
		}
		message.Title("To inspect/deploy/pull:", "")
		message.ZarfCommand("package inspect %s %s", helpers.OCIURLPrefix+remote.Repo().Reference.String(), flags)
		message.ZarfCommand("package deploy %s %s", helpers.OCIURLPrefix+remote.Repo().Reference.String(), flags)
		message.ZarfCommand("package pull %s %s", helpers.OCIURLPrefix+remote.Repo().Reference.String(), flags)
	} else {
		// Use the output path if the user specified it.
		packageName := filepath.Join(pc.createOpts.Output, utils.GetPackageName(*loadedPkg, pc.createOpts.DifferentialData))

		// Try to remove the package if it already exists.
		_ = os.Remove(packageName)

		// Create the package tarball.
		if err := dst.ArchivePackage(packageName, pc.createOpts.MaxPackageSizeMB); err != nil {
			return fmt.Errorf("unable to archive package: %w", err)
		}
	}

	// Output the SBOM files into a directory if specified.
	if pc.createOpts.ViewSBOM || pc.createOpts.SBOMOutputDir != "" {
		outputSBOM := pc.createOpts.SBOMOutputDir
		var sbomDir string
		if err := dst.SBOMs.Unarchive(); err != nil {
			return fmt.Errorf("unable to unarchive SBOMs: %w", err)
		}
		sbomDir = dst.SBOMs.Path

		if outputSBOM != "" {
			out, err := utils.OutputSBOMFiles(sbomDir, outputSBOM, loadedPkg.Metadata.Name)
			if err != nil {
				return err
			}
			sbomDir = out
		}

		if pc.createOpts.ViewSBOM {
			sbom.ViewSBOMFiles(sbomDir)
		}
	}
	return nil
}

func (pc *PackageCreator) processExtensions(pkg *types.ZarfPackage, layout *layout.PackagePaths) (extendedPkg *types.ZarfPackage, err error) {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range pkg.Components {
		componentPaths, err := layout.Components.Create(c)
		if err != nil {
			return nil, err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Run(pkg.Metadata.YOLO, componentPaths, c); err != nil {
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

func (pc *PackageCreator) addComponent(component types.ZarfComponent, dst *layout.PackagePaths) error {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return err
	}

	onCreate := component.Actions.OnCreate
	if err := actions.Run(pc.cfg, onCreate.Defaults, onCreate.Before, nil); err != nil {
		return fmt.Errorf("unable to run component before action: %w", err)
	}

	// If any helm charts are defined, process them.
	for _, chart := range component.Charts {
		helmCfg := helm.New(chart, componentPaths.Charts, componentPaths.Values)
		if err := helmCfg.PackageChart(componentPaths.Charts); err != nil {
			return err
		}
	}

	for filesIdx, file := range component.Files {
		message.Debugf("Loading %#v", file)

		rel := filepath.Join(layout.FilesDir, strconv.Itoa(filesIdx), filepath.Base(file.Target))
		dst := filepath.Join(componentPaths.Base, rel)
		destinationDir := filepath.Dir(dst)

		if helpers.IsURL(file.Source) {
			if file.ExtractPath != "" {
				// get the compressedFileName from the source
				compressedFileName, err := helpers.ExtractBasePathFromURL(file.Source)
				if err != nil {
					return fmt.Errorf(lang.ErrFileNameExtract, file.Source, err.Error())
				}

				compressedFile := filepath.Join(componentPaths.Temp, compressedFileName)

				// If the file is an archive, download it to the componentPath.Temp
				if err := utils.DownloadToFile(file.Source, compressedFile, component.DeprecatedCosignKeyPath); err != nil {
					return fmt.Errorf(lang.ErrDownloading, file.Source, err.Error())
				}

				err = archiver.Extract(compressedFile, file.ExtractPath, destinationDir)
				if err != nil {
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, compressedFileName, err.Error())
				}
			} else {
				if err := utils.DownloadToFile(file.Source, dst, component.DeprecatedCosignKeyPath); err != nil {
					return fmt.Errorf(lang.ErrDownloading, file.Source, err.Error())
				}
			}
		} else {
			if file.ExtractPath != "" {
				if err := archiver.Extract(file.Source, file.ExtractPath, destinationDir); err != nil {
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, file.Source, err.Error())
				}
			} else {
				if err := utils.CreatePathAndCopy(file.Source, dst); err != nil {
					return fmt.Errorf("unable to copy file %s: %w", file.Source, err)
				}
			}
		}

		if file.ExtractPath != "" {
			// Make sure dst reflects the actual file or directory.
			updatedExtractedFileOrDir := filepath.Join(destinationDir, file.ExtractPath)
			if updatedExtractedFileOrDir != dst {
				if err := os.Rename(updatedExtractedFileOrDir, dst); err != nil {
					return fmt.Errorf(lang.ErrWritingFile, dst, err)
				}
			}
		}

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

			if helpers.IsURL(data.Source) {
				if err := utils.DownloadToFile(data.Source, dst, component.DeprecatedCosignKeyPath); err != nil {
					return fmt.Errorf(lang.ErrDownloading, data.Source, err.Error())
				}
			} else {
				if err := utils.CreatePathAndCopy(data.Source, dst); err != nil {
					return fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
				}
			}
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
		for _, manifest := range component.Manifests {
			for fileIdx, path := range manifest.Files {
				rel := filepath.Join(layout.ManifestsDir, fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
				dst := filepath.Join(componentPaths.Base, rel)

				// Copy manifests without any processing.
				spinner.Updatef("Copying manifest %s", path)
				if helpers.IsURL(path) {
					if err := utils.DownloadToFile(path, dst, component.DeprecatedCosignKeyPath); err != nil {
						return fmt.Errorf(lang.ErrDownloading, path, err.Error())
					}
				} else {
					if err := utils.CreatePathAndCopy(path, dst); err != nil {
						return fmt.Errorf("unable to copy manifest %s: %w", path, err)
					}
				}
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
			}
		}
		spinner.Success()
	}

	// Load all specified git repos.
	if len(component.Repos) > 0 {
		spinner := message.NewProgressSpinner("Loading %d git repos", len(component.Repos))
		defer spinner.Stop()

		for _, url := range component.Repos {
			// Pull all the references if there is no `@` in the string.
			gitCfg := git.NewWithSpinner(types.GitServerInfo{}, spinner)
			if err := gitCfg.Pull(url, componentPaths.Repos, false); err != nil {
				return fmt.Errorf("unable to pull git repo %s: %w", url, err)
			}
		}
		spinner.Success()
	}

	if err := actions.Run(pc.cfg, onCreate.Defaults, onCreate.After, nil); err != nil {
		return fmt.Errorf("unable to run component after action: %w", err)
	}

	return nil
}

func (pc *PackageCreator) getFilesToSBOM(component types.ZarfComponent, dst *layout.PackagePaths) (*layout.ComponentSBOM, error) {
	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return nil, err
	}
	// Create an struct to hold the SBOM information for this component.
	componentSBOM := &layout.ComponentSBOM{
		Files:     []string{},
		Component: componentPaths,
	}

	appendSBOMFiles := func(path string) {
		if utils.IsDir(path) {
			files, _ := utils.RecursiveFileList(path, nil, false)
			componentSBOM.Files = append(componentSBOM.Files, files...)
		} else {
			componentSBOM.Files = append(componentSBOM.Files, path)
		}
	}

	for filesIdx, file := range component.Files {
		path := filepath.Join(componentPaths.Files, strconv.Itoa(filesIdx), filepath.Base(file.Target))
		appendSBOMFiles(path)
	}

	for dataIdx, data := range component.DataInjections {
		path := filepath.Join(componentPaths.DataInjections, strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))

		appendSBOMFiles(path)
	}

	return componentSBOM, nil
}
