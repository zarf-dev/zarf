// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/pkg/oci"
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
	"github.com/defenseunicorns/zarf/src/pkg/packager/actions"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/zoci"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

var (
	// verify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)
)

// PackageCreator provides methods for creating normal (not skeleton) Zarf packages.
type PackageCreator struct {
	createOpts types.ZarfCreateOptions
}

func updateRelativeDifferentialPackagePath(path string, cwd string) string {
	if path != "" && !filepath.IsAbs(path) && !helpers.IsURL(path) {
		return filepath.Join(cwd, path)
	}
	return path
}

// NewPackageCreator returns a new PackageCreator.
func NewPackageCreator(createOpts types.ZarfCreateOptions, cwd string) *PackageCreator {
	createOpts.DifferentialPackagePath = updateRelativeDifferentialPackagePath(createOpts.DifferentialPackagePath, cwd)
	return &PackageCreator{createOpts}
}

// LoadPackageDefinition loads and configures a zarf.yaml file during package create.
func (pc *PackageCreator) LoadPackageDefinition(src *layout.PackagePaths) (pkg types.ZarfPackage, warnings []string, err error) {
	pkg, warnings, err = src.ReadZarfYAML()
	if err != nil {
		return types.ZarfPackage{}, nil, err
	}

	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)

	// Compose components into a single zarf.yaml file
	pkg, composeWarnings, err := ComposeComponents(pkg, pc.createOpts.Flavor)
	if err != nil {
		return types.ZarfPackage{}, nil, err
	}

	warnings = append(warnings, composeWarnings...)

	// After components are composed, template the active package.
	pkg, templateWarnings, err := FillActiveTemplate(pkg, pc.createOpts.SetVariables)
	if err != nil {
		return types.ZarfPackage{}, nil, fmt.Errorf("unable to fill values in template: %w", err)
	}

	warnings = append(warnings, templateWarnings...)

	// After templates are filled process any create extensions
	pkg.Components, err = pc.processExtensions(pkg.Components, src, pkg.Metadata.YOLO)
	if err != nil {
		return types.ZarfPackage{}, nil, err
	}

	// If we are creating a differential package, remove duplicate images and repos.
	if pc.createOpts.DifferentialPackagePath != "" {
		pkg.Build.Differential = true

		diffData, err := loadDifferentialData(pc.createOpts.DifferentialPackagePath)
		if err != nil {
			return types.ZarfPackage{}, nil, err
		}

		pkg.Build.DifferentialPackageVersion = diffData.DifferentialPackageVersion

		versionsMatch := diffData.DifferentialPackageVersion == pkg.Metadata.Version
		if versionsMatch {
			return types.ZarfPackage{}, nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}

		noVersionSet := diffData.DifferentialPackageVersion == "" || pkg.Metadata.Version == ""
		if noVersionSet {
			return types.ZarfPackage{}, nil, errors.New(lang.PkgCreateErrDifferentialNoVersion)
		}

		filter := filters.ByDifferentialData(diffData)
		pkg.Components, err = filter.Apply(pkg)
		if err != nil {
			return types.ZarfPackage{}, nil, err
		}
	}

	if err := pkg.Validate(); err != nil {
		return types.ZarfPackage{}, nil, err
	}

	return pkg, warnings, nil
}

// Assemble assembles all of the package assets into Zarf's tmp directory layout.
func (pc *PackageCreator) Assemble(dst *layout.PackagePaths, components []types.ZarfComponent, arch string) error {
	var imageList []transform.Image

	skipSBOMFlagUsed := pc.createOpts.SkipSBOM
	componentSBOMs := map[string]*layout.ComponentSBOM{}

	for _, component := range components {
		onCreate := component.Actions.OnCreate

		onFailure := func() {
			if err := actions.Run(onCreate.Defaults, onCreate.OnFailure, nil); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}

		if err := pc.addComponent(component, dst); err != nil {
			onFailure()
			return fmt.Errorf("unable to add component %q: %w", component.Name, err)
		}

		if err := actions.Run(onCreate.Defaults, onCreate.OnSuccess, nil); err != nil {
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
	rs := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(rs)
	rnd.Shuffle(len(imageList), func(i, j int) { imageList[i], imageList[j] = imageList[j], imageList[i] })
	var sbomImageList []transform.Image

	// Images are handled separately from other component assets.
	if len(imageList) > 0 {
		message.HeaderInfof("📦 PACKAGE IMAGES")

		dst.AddImages()

		ctx := context.TODO()

		pullCfg := images.PullConfig{
			DestinationDirectory: dst.Images.Base,
			ImageList:            imageList,
			Arch:                 arch,
			RegistryOverrides:    pc.createOpts.RegistryOverrides,
			CacheDirectory:       filepath.Join(config.GetAbsCachePath(), layout.ImagesDir),
		}

		pulled, err := images.Pull(ctx, pullCfg)
		if err != nil {
			return err
		}

		for info, img := range pulled {
			if err := dst.Images.AddV1Image(img); err != nil {
				return err
			}
			ok, err := utils.HasImageLayers(img)
			if err != nil {
				return fmt.Errorf("failed to validate %s is an image and not an artifact: %w", info, err)
			}
			if ok {
				sbomImageList = append(sbomImageList, info)
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
func (pc *PackageCreator) Output(dst *layout.PackagePaths, pkg *types.ZarfPackage) (err error) {
	// Process the component directories into compressed tarballs
	// NOTE: This is purposefully being done after the SBOM cataloging
	for _, component := range pkg.Components {
		// Make the component a tar archive
		if err := dst.Components.Archive(component, true); err != nil {
			return fmt.Errorf("unable to archive component: %s", err.Error())
		}
	}

	// Calculate all the checksums
	pkg.Metadata.AggregateChecksum, err = dst.GenerateChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for the package: %w", err)
	}

	if err := recordPackageMetadata(pkg, pc.createOpts); err != nil {
		return err
	}

	if err := utils.WriteYaml(dst.ZarfYAML, pkg, helpers.ReadUser); err != nil {
		return fmt.Errorf("unable to write zarf.yaml: %w", err)
	}

	// Sign the package if a key has been provided
	if err := dst.SignPackage(pc.createOpts.SigningKeyPath, pc.createOpts.SigningKeyPassword, !config.CommonOptions.Confirm); err != nil {
		return err
	}

	// Create a remote ref + client for the package (if output is OCI)
	// then publish the package to the remote.
	if helpers.IsOCIURL(pc.createOpts.Output) {
		ref, err := zoci.ReferenceFromMetadata(pc.createOpts.Output, &pkg.Metadata, &pkg.Build)
		if err != nil {
			return err
		}
		remote, err := zoci.NewRemote(ref, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			return err
		}

		ctx := context.TODO()
		err = remote.PublishPackage(ctx, pkg, dst, config.CommonOptions.OCIConcurrency)
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
		packageName := fmt.Sprintf("%s%s", sources.NameFromMetadata(pkg, pc.createOpts.IsSkeleton), sources.PkgSuffix(pkg.Metadata.Uncompressed))
		tarballPath := filepath.Join(pc.createOpts.Output, packageName)

		// Try to remove the package if it already exists.
		_ = os.Remove(tarballPath)

		// Create the package tarball.
		if err := dst.ArchivePackage(tarballPath, pc.createOpts.MaxPackageSizeMB); err != nil {
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
			out, err := dst.SBOMs.OutputSBOMFiles(outputSBOM, pkg.Metadata.Name)
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

func (pc *PackageCreator) processExtensions(components []types.ZarfComponent, layout *layout.PackagePaths, isYOLO bool) (processedComponents []types.ZarfComponent, err error) {
	// Create component paths and process extensions for each component.
	for _, c := range components {
		componentPaths, err := layout.Components.Create(c)
		if err != nil {
			return nil, err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Run(isYOLO, componentPaths, c); err != nil {
				return nil, fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		processedComponents = append(processedComponents, c)
	}

	return processedComponents, nil
}

func (pc *PackageCreator) addComponent(component types.ZarfComponent, dst *layout.PackagePaths) error {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return err
	}

	onCreate := component.Actions.OnCreate
	if err := actions.Run(onCreate.Defaults, onCreate.Before, nil); err != nil {
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
				if err := helpers.CreatePathAndCopy(file.Source, dst); err != nil {
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
			if err := helpers.SHAsMatch(dst, file.Shasum); err != nil {
				return err
			}
		}

		if file.Executable || helpers.IsDir(dst) {
			_ = os.Chmod(dst, helpers.ReadWriteExecuteUser)
		} else {
			_ = os.Chmod(dst, helpers.ReadWriteUser)
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
				if err := helpers.CreatePathAndCopy(data.Source, dst); err != nil {
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
					if err := helpers.CreatePathAndCopy(path, dst); err != nil {
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

	if err := actions.Run(onCreate.Defaults, onCreate.After, nil); err != nil {
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
		if helpers.IsDir(path) {
			files, _ := helpers.RecursiveFileList(path, nil, false)
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
