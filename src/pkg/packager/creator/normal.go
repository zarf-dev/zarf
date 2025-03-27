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

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/internal/packager/sbom"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
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
func (pc *PackageCreator) LoadPackageDefinition(ctx context.Context, src *layout.PackagePaths) (pkg v1alpha1.ZarfPackage, warnings []string, err error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("start loading package definiton", "src", src.ZarfYAML)

	pkg, warnings, err = src.ReadZarfYAML()
	if err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	pkg.Metadata.Architecture = config.GetArch(pkg.Metadata.Architecture)

	// Compose components into a single zarf.yaml file
	pkg, composeWarnings, err := ComposeComponents(ctx, pkg, pc.createOpts.Flavor)
	if err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}
	warnings = append(warnings, composeWarnings...)

	// After components are composed, template the active package.
	pkg, templateWarnings, err := FillActiveTemplate(ctx, pkg, pc.createOpts.SetVariables)
	if err != nil {
		return v1alpha1.ZarfPackage{}, nil, fmt.Errorf("unable to fill values in template: %w", err)
	}

	warnings = append(warnings, templateWarnings...)

	// If we are creating a differential package, remove duplicate images and repos.
	if pc.createOpts.DifferentialPackagePath != "" {
		pkg.Build.Differential = true

		diffData, err := loadDifferentialData(ctx, pc.createOpts.DifferentialPackagePath)
		if err != nil {
			return v1alpha1.ZarfPackage{}, nil, err
		}

		pkg.Build.DifferentialPackageVersion = diffData.DifferentialPackageVersion

		versionsMatch := diffData.DifferentialPackageVersion == pkg.Metadata.Version
		if versionsMatch {
			return v1alpha1.ZarfPackage{}, nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}

		noVersionSet := diffData.DifferentialPackageVersion == "" || pkg.Metadata.Version == ""
		if noVersionSet {
			return v1alpha1.ZarfPackage{}, nil, errors.New(lang.PkgCreateErrDifferentialNoVersion)
		}

		filter := filters.ByDifferentialData(diffData)
		pkg.Components, err = filter.Apply(pkg)
		if err != nil {
			return v1alpha1.ZarfPackage{}, nil, err
		}
	}

	if err := Validate(pkg, pc.createOpts.BaseDir, pc.createOpts.SetVariables); err != nil {
		return v1alpha1.ZarfPackage{}, nil, err
	}

	l.Debug("done loading package definition", "src", src.ZarfYAML, "duration", time.Since(start))
	return pkg, warnings, nil
}

// Assemble copies all package assets into Zarf's tmp directory layout.
func (pc *PackageCreator) Assemble(ctx context.Context, dst *layout.PackagePaths, components []v1alpha1.ZarfComponent, arch string) error {
	var imageList []transform.Image
	l := logger.From(ctx)

	skipSBOMFlagUsed := pc.createOpts.SkipSBOM
	componentSBOMs := map[string]*layout.ComponentSBOM{}

	for _, component := range components {
		onCreate := component.Actions.OnCreate

		onFailure := func() {
			if err := actions.Run(ctx, onCreate.Defaults, onCreate.OnFailure, nil); err != nil {
				// TODO(mkcp): Remove message on logger release
				message.Debugf("unable to run component failure action: %s", err.Error())
				l.Debug("unable to run component failure action", "error", err.Error())
			}
		}

		if err := pc.addComponent(ctx, component, dst); err != nil {
			onFailure()
			return fmt.Errorf("unable to add component %q: %w", component.Name, err)
		}

		// TODO(mkcp): Migrate to logger
		if err := actions.Run(ctx, onCreate.Defaults, onCreate.OnSuccess, nil); err != nil {
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

	rs := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(rs)
	rnd.Shuffle(len(imageList), func(i, j int) { imageList[i], imageList[j] = imageList[j], imageList[i] })
	var sbomImageList []transform.Image

	// Images are handled separately from other component assets.
	if len(imageList) > 0 {
		// TODO(mkcp): Remove message on logger release
		message.HeaderInfof("📦 PACKAGE IMAGES")
		dst.AddImages()

		cachePath, err := config.GetAbsCachePath()
		if err != nil {
			return err
		}
		pullCfg := images.PullConfig{
			DestinationDirectory: dst.Images.Base,
			ImageList:            imageList,
			Arch:                 arch,
			OCIConcurrency:       config.CommonOptions.OCIConcurrency,
			RegistryOverrides:    pc.createOpts.RegistryOverrides,
			CacheDirectory:       filepath.Join(cachePath, layout.ImagesDir),
		}

		_, err = images.Pull(ctx, pullCfg)
		if err != nil {
			return err
		}

		pulled := map[transform.Image]v1.Image{}
		for _, ref := range imageList {
			image, err := utils.LoadOCIImage(dst.Images.Base, ref)
			if err != nil {
				return err
			}
			pulled[ref] = image
		}

		for info, img := range pulled {
			if err := dst.Images.AddV1Image(img); err != nil {
				return err
			}
			ok, err := utils.OnlyHasImageLayers(img)
			if err != nil {
				return fmt.Errorf("failed to validate %s is an image and not an artifact: %w", info, err)
			}
			if ok {
				sbomImageList = append(sbomImageList, info)
			}
		}

		// Sort images index to make build reproducible.
		err = utils.SortImagesIndex(dst.Images.Base)
		if err != nil {
			return err
		}
	}

	// Ignore SBOM creation if the flag is set.
	if skipSBOMFlagUsed {
		// TODO(mkcp): Remove message on logger release
		message.Debug("Skipping image SBOM processing per --skip-sbom flag")
		l.Debug("skipping image SBOM processing per --skip-sbom flag")
		return nil
	}

	dst.AddSBOMs()
	if err := sbom.Catalog(ctx, componentSBOMs, sbomImageList, dst); err != nil {
		return fmt.Errorf("unable to create an SBOM catalog for the package: %w", err)
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
func (pc *PackageCreator) Output(ctx context.Context, dst *layout.PackagePaths, pkg *v1alpha1.ZarfPackage) (err error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Debug("starting package output", "kind", pkg.Kind)

	// Process the component directories into compressed tarballs
	// NOTE: This is purposefully being done after the SBOM cataloging
	for _, component := range pkg.Components {
		// Make the component a tar archive
		if err := dst.Components.Archive(ctx, component, true); err != nil {
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
		remote, err := zoci.NewRemote(ctx, ref, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			return err
		}
		err = remote.PublishPackage(ctx, pkg, dst, config.CommonOptions.OCIConcurrency)
		if err != nil {
			return fmt.Errorf("unable to publish package: %w", err)
		}
		message.HorizontalRule()
		flags := []string{}
		if config.CommonOptions.PlainHTTP {
			flags = append(flags, "--plain-http")
		}
		if config.CommonOptions.InsecureSkipTLSVerify {
			flags = append(flags, "--insecure-skip-tls-verify")
		}
		// TODO(mkcp): Remove message on logger release
		message.Title("To inspect/deploy/pull:", "")
		message.ZarfCommand("package inspect %s %s", helpers.OCIURLPrefix+remote.Repo().Reference.String(), strings.Join(flags, " "))
		message.ZarfCommand("package deploy %s %s", helpers.OCIURLPrefix+remote.Repo().Reference.String(), strings.Join(flags, " "))
		message.ZarfCommand("package pull %s %s", helpers.OCIURLPrefix+remote.Repo().Reference.String(), strings.Join(flags, " "))
	} else {
		// Use the output path if the user specified it.
		packageName := fmt.Sprintf("%s%s", sources.NameFromMetadata(pkg, pc.createOpts.IsSkeleton), sources.PkgSuffix(pkg.Metadata.Uncompressed))
		tarballPath := filepath.Join(pc.createOpts.Output, packageName)

		// remove existing package with the same name
		err = os.Remove(tarballPath)
		// user only cares about this error if file exists and the remove failed
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.From(ctx).Error(err.Error())
		}

		// Create the package tarball.
		if err := dst.ArchivePackage(ctx, tarballPath, pc.createOpts.MaxPackageSizeMB); err != nil {
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
			err := sbom.ViewSBOMFiles(ctx, sbomDir)
			if err != nil {
				return err
			}
		}
	}
	l.Debug("done package output", "kind", pkg.Kind, "duration", time.Since(start))
	return nil
}

// TODO(mkcp): Refactor addComponent to better segment component handling logic by its type. There's also elaborate
// if/elses that can be de-nested.
func (pc *PackageCreator) addComponent(ctx context.Context, component v1alpha1.ZarfComponent, dst *layout.PackagePaths) error {
	l := logger.From(ctx)
	// TODO(mkcp): Remove message on logger release
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))
	l.Info("adding component", "name", component.Name)
	start := time.Now()

	componentPaths, err := dst.Components.Create(component)
	if err != nil {
		return err
	}

	onCreate := component.Actions.OnCreate
	if err := actions.Run(ctx, onCreate.Defaults, onCreate.Before, nil); err != nil {
		return fmt.Errorf("unable to run component before action: %w", err)
	}

	// If any helm charts are defined, process them.
	for _, chart := range component.Charts {
		helmCfg := helm.New(chart, componentPaths.Charts, componentPaths.Values)
		if err := helmCfg.PackageChart(ctx, componentPaths.Charts); err != nil {
			return err
		}
	}

	for filesIdx, file := range component.Files {
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
				if err := utils.DownloadToFile(ctx, file.Source, compressedFile, component.DeprecatedCosignKeyPath); err != nil {
					return fmt.Errorf(lang.ErrDownloading, file.Source, err.Error())
				}

				err = archiver.Extract(compressedFile, file.ExtractPath, destinationDir)
				if err != nil {
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, compressedFileName, err.Error())
				}
			} else {
				if err := utils.DownloadToFile(ctx, file.Source, dst, component.DeprecatedCosignKeyPath); err != nil {
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

	// Run data injections
	injectionsCount := len(component.DataInjections)
	if injectionsCount > 0 {
		injectStart := time.Now()
		// TODO(mkcp): Remove message on logger release
		spinner := message.NewProgressSpinner("Loading data injections")
		defer spinner.Stop()
		l.Info("data injections found, running", "injections", injectionsCount)

		for dataIdx, data := range component.DataInjections {
			target := data.Target
			spinner.Updatef("Copying data injection %s for %s", target.Path, target.Selector)
			l.Info("copying data injection", "path", target.Path, "selector", target.Selector)

			rel := filepath.Join(layout.DataInjectionsDir, strconv.Itoa(dataIdx), filepath.Base(target.Path))
			dst := filepath.Join(componentPaths.Base, rel)

			if helpers.IsURL(data.Source) {
				err := utils.DownloadToFile(ctx, data.Source, dst, component.DeprecatedCosignKeyPath)
				if err != nil {
					return fmt.Errorf(lang.ErrDownloading, data.Source, err.Error())
				}
			} else {
				if err := helpers.CreatePathAndCopy(data.Source, dst); err != nil {
					return fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
				}
			}
		}
		spinner.Success()
		l.Debug("done loading data injections", "duration", time.Since(injectStart))
	}

	// Process k8s manifests
	if len(component.Manifests) > 0 {
		manifestStart := time.Now()
		// Get the proper count of total manifests to add.
		manifestCount := 0

		for _, manifest := range component.Manifests {
			manifestCount += len(manifest.Files)
			manifestCount += len(manifest.Kustomizations)
		}

		// TODO(mkcp): Remove message on logger release
		spinner := message.NewProgressSpinner("Loading %d K8s manifests", manifestCount)
		defer spinner.Stop()
		l.Info("processing k8s manifests", "component", component.Name, "manifests", manifestCount)

		// Iterate over all manifests.
		for _, manifest := range component.Manifests {
			for fileIdx, path := range manifest.Files {
				rel := filepath.Join(layout.ManifestsDir, fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
				dst := filepath.Join(componentPaths.Base, rel)

				// Copy manifests without any processing.
				// TODO(mkcp): Remove message on logger release
				spinner.Updatef("Copying manifest %s", path)
				l.Info("copying manifest", "path", path)
				if helpers.IsURL(path) {
					if err := utils.DownloadToFile(ctx, path, dst, component.DeprecatedCosignKeyPath); err != nil {
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
				// TODO(mkcp): Remove message on logger release
				spinner.Updatef("Building kustomization for %s", path)
				l.Info("building kustomization", "path", path)

				kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
				rel := filepath.Join(layout.ManifestsDir, kname)
				dst := filepath.Join(componentPaths.Base, rel)

				if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
					return fmt.Errorf("unable to build kustomization %s: %w", path, err)
				}
			}
		}
		spinner.Success()
		l.Debug("done processing k8s manifests",
			"component", component.Name,
			"duration", time.Since(manifestStart))
	}

	// Load all specified git repos.
	reposCount := len(component.Repos)
	if reposCount > 0 {
		reposStart := time.Now()
		// TODO(mkcp): Remove message on logger release
		spinner := message.NewProgressSpinner("Loading %d git repos", len(component.Repos))
		defer spinner.Stop()
		l.Info("loading git repos", "component", component.Name, "repos", reposCount)

		for _, url := range component.Repos {
			// Pull all the references if there is no `@` in the string.
			_, err := git.Clone(ctx, componentPaths.Repos, url, false)
			if err != nil {
				return fmt.Errorf("unable to pull git repo %s: %w", url, err)
			}
		}
		spinner.Success()
		l.Debug("done loading git repos", "component", component.Name, "duration", time.Since(reposStart))
	}

	if err := actions.Run(ctx, onCreate.Defaults, onCreate.After, nil); err != nil {
		return fmt.Errorf("unable to run component after action: %w", err)
	}

	l.Debug("done adding component", "name", component.Name, "duration", time.Since(start))
	return nil
}

func (pc *PackageCreator) getFilesToSBOM(component v1alpha1.ZarfComponent, dst *layout.PackagePaths) (*layout.ComponentSBOM, error) {
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
