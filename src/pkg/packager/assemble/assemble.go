// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package assemble builds a Zarf package on disk: it resolves each component's
// charts, manifests, files, repos, images and data injections into the layout that
// package layout reads back. It owns the build-time dependencies (helm, git,
// kustomize, image pulls, SBOM generation) so `layout` can focus on reading and naming.
package assemble

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/types"
)

// AssembleOptions are the options for creating a package from a package object
type AssembleOptions struct {
	// Flavor causes the package to only include components with a matching `.components[x].only.flavor` or no flavor `.components[x].only.flavor` specified
	Flavor string
	// RegistryOverrides overrides the basepath of an OCI image with a path to a different registry
	RegistryOverrides  []images.RegistryOverride
	SigningKeyPath     string
	SigningKeyPassword string
	SkipSBOM           bool
	// When DifferentialPackage is set the zarf package created only includes images and repos not in the differential package
	DifferentialPackage v1alpha1.ZarfPackage
	OCIConcurrency      int
	// CachePath is the path to the Zarf cache, used to cache images and charts
	CachePath string
	// WithBuildMachineInfo includes build machine information (hostname and username) in the package metadata
	WithBuildMachineInfo bool
	types.RemoteOptions
}

// AssemblePackage takes a resolved package and returns a package layout with all the resources collected.
func AssemblePackage(ctx context.Context, resolvedPackage load.ResolvedPackage, packagePath string, opts AssembleOptions) (*layout.PackageLayout, error) {
	l := logger.From(ctx)
	l.Info("assembling package", "path", packagePath)

	definition := resolvedPackage.PackageDefinition
	pkg := definition.AsV1alpha1()
	importedSchemas := resolvedPackage.ImportedSchemas
	if err := validateImageArchivesNoDuplicates(pkg.Components); err != nil {
		return nil, err
	}

	if opts.DifferentialPackage.Metadata.Name != "" {
		l.Debug("creating differential package", "differential", opts.DifferentialPackage)
		versionsMatch := opts.DifferentialPackage.Metadata.Version == pkg.Metadata.Version
		if versionsMatch {
			return nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		noVersionSet := opts.DifferentialPackage.Metadata.Version == "" || pkg.Metadata.Version == ""
		if noVersionSet {
			return nil, errors.New(lang.PkgCreateErrDifferentialNoVersion)
		}
		originalAPIVersion := definition.OriginalAPIVersion()
		differentialAPIVersion := opts.DifferentialPackage.Build.GetOriginalAPIVersion()
		if originalAPIVersion != differentialAPIVersion {
			return nil, fmt.Errorf("%s: package apiVersion %s, differential package apiVersion %s", lang.PkgCreateErrDifferentialAPIVersion, originalAPIVersion, differentialAPIVersion)
		}
		updatedDefinition, err := applyDifferentialResources(definition, api.NewPackageDefinitionFromV1alpha1(opts.DifferentialPackage))
		if err != nil {
			return nil, err
		}
		definition = updatedDefinition
		pkg = definition.AsV1alpha1()
		definition.SetDifferentialBuild(opts.DifferentialPackage.Metadata.Version)
	}

	buildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	for _, component := range pkg.Components {
		err := assemblePackageComponent(ctx, component, packagePath, buildPath, opts.CachePath, opts.RemoteOptions)
		if err != nil {
			return nil, err
		}
	}

	componentImages := []transform.Image{}
	manifests := []images.PulledImage{}
	for _, component := range pkg.Components {
		for _, imageArchive := range component.ImageArchives {
			if !filepath.IsAbs(imageArchive.Path) {
				imageArchive.Path = filepath.Join(packagePath, imageArchive.Path)
			}

			archiveImageManifests, err := images.Unpack(ctx, imageArchive, filepath.Join(buildPath, layout.ImagesDir), pkg.Metadata.Architecture)
			if err != nil {
				return nil, err
			}
			manifests = append(manifests, archiveImageManifests...)
		}
		for _, src := range component.Images {
			refInfo, err := transform.ParseImageRef(src)
			if err != nil {
				return nil, fmt.Errorf("failed to create ref for image %s: %w", src, err)
			}
			if slices.Contains(componentImages, refInfo) {
				continue
			}
			componentImages = append(componentImages, refInfo)
		}
	}
	sbomImageList := []transform.Image{}
	if len(componentImages) > 0 {
		pullOpts := images.PullOptions{
			OCIConcurrency:        opts.OCIConcurrency,
			Arch:                  pkg.Metadata.Architecture,
			RegistryOverrides:     opts.RegistryOverrides,
			CacheDirectory:        filepath.Join(opts.CachePath, layout.ImagesDir),
			InsecureSkipTLSVerify: opts.RemoteOptions.InsecureSkipTLSVerify,
			PlainHTTP:             opts.RemoteOptions.PlainHTTP,
		}
		imageManifests, err := images.Pull(ctx, componentImages, filepath.Join(buildPath, layout.ImagesDir), pullOpts)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, imageManifests...)
	}

	for _, pulled := range manifests {
		sbomImageList = append(sbomImageList, pulled.Image)

		// Sort images index to make build reproducible.
		err = utils.SortImagesIndex(filepath.Join(buildPath, layout.ImagesDir))
		if err != nil {
			return nil, err
		}
	}

	l.Info("composed components successfully")

	if !opts.SkipSBOM && pkg.IsSBOMAble() {
		l.Info("generating SBOM")
		err := generateSBOM(ctx, pkg, buildPath, sbomImageList, opts.CachePath)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SBOM: %w", err)
		}
	}

	l.Debug("merging values files to package", "files", pkg.Values.Files)
	if err = mergeAndWriteValuesFile(ctx, pkg.Values.Files, packagePath, buildPath); err != nil {
		return nil, err
	}

	if err = mergeAndWriteValuesSchema(ctx, pkg.Values.Schema, importedSchemas, packagePath, buildPath); err != nil {
		return nil, err
	}

	if err = createDocumentationTar(pkg, packagePath, buildPath); err != nil {
		return nil, err
	}

	checksumContent, checksumSha, err := GetChecksum(buildPath)
	if err != nil {
		return nil, err
	}
	checksumPath := filepath.Join(buildPath, layout.Checksums)
	err = os.WriteFile(checksumPath, []byte(checksumContent), helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}
	if err = recordPackageMetadata(&definition, opts.Flavor, opts.RegistryOverrides, opts.WithBuildMachineInfo, buildPath, checksumSha); err != nil {
		return nil, err
	}

	err = layout.WritePackageDefinition(filepath.Join(buildPath, layout.ZarfYAML), definition)
	if err != nil {
		return nil, err
	}

	// skip verification on package creation
	pkgLayout, err := layout.LoadFromDir(ctx, buildPath, layout.PackageLayoutOptions{VerificationStrategy: layout.VerifyNever})
	if err != nil {
		return nil, err
	}

	// Sign the package with the provided options
	signOpts := signing.DefaultSignBlobOptions()
	signOpts.Key = opts.SigningKeyPath
	signOpts.Password = opts.SigningKeyPassword

	err = pkgLayout.SignPackage(ctx, signOpts)
	if err != nil {
		return nil, err
	}

	return pkgLayout, nil
}

// AssembleSkeletonOptions are the options for creating a skeleton package
type AssembleSkeletonOptions struct {
	SigningKeyPath       string
	SigningKeyPassword   string
	Flavor               string
	WithBuildMachineInfo bool
}

// AssembleSkeleton creates a skeleton package and returns the path to the created package.
func AssembleSkeleton(ctx context.Context, resolvedPackage load.ResolvedPackage, packagePath string, opts AssembleSkeletonOptions) (*layout.PackageLayout, error) {
	definition := resolvedPackage.PackageDefinition
	definition.SetMetadataArchitecture(v1alpha1.SkeletonArch)
	pkg := definition.AsV1alpha1()
	importedSchemas := resolvedPackage.ImportedSchemas

	// Creating skeleton packages with the values feature is not yet supported
	if len(pkg.Values.Files) > 0 || pkg.Values.Schema != "" || len(importedSchemas) > 0 {
		return nil, errors.New("creating skeleton packages with the values feature is not yet supported")
	}

	buildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}

	if err = createDocumentationTar(pkg, packagePath, buildPath); err != nil {
		return nil, err
	}

	// To remove the flavor value, as the flavor is configured by the tag uploaded to the registry
	//   example:
	//     url: oci://ghcr.io/zarf-dev/packages/init:v0.58.0-upstream
	//     is indicating that you are importing the "upstream" flavor of the zarf init package
	for i := 0; i < len(pkg.Components); i++ {
		pkg.Components[i].Only.Flavor = ""
		err := assembleSkeletonComponent(ctx, pkg.Components[i], packagePath, buildPath)
		if err != nil {
			return nil, err
		}
	}

	checksumContent, checksumSha, err := GetChecksum(buildPath)
	if err != nil {
		return nil, err
	}
	checksumPath := filepath.Join(buildPath, layout.Checksums)
	err = os.WriteFile(checksumPath, []byte(checksumContent), helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}
	// PackageDefinition does not expose component flavor mutations, so retain them
	// while moving package metadata updates to the generic definition.
	definition = api.NewPackageDefinitionFromV1alpha1(pkg)

	if err = recordPackageMetadata(&definition, opts.Flavor, nil, opts.WithBuildMachineInfo, buildPath, checksumSha); err != nil {
		return nil, err
	}

	if err = layout.WritePackageDefinition(filepath.Join(buildPath, layout.ZarfYAML), definition); err != nil {
		return nil, err
	}

	layoutOpts := layout.PackageLayoutOptions{
		VerificationStrategy: layout.VerifyNever,
		IsPartial:            false,
	}
	pkgLayout, err := layout.LoadFromDir(ctx, buildPath, layoutOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to load skeleton: %w", err)
	}

	// Sign the package with the provided options
	signOpts := signing.DefaultSignBlobOptions()
	signOpts.Key = opts.SigningKeyPath
	signOpts.Password = opts.SigningKeyPassword

	err = pkgLayout.SignPackage(ctx, signOpts)
	if err != nil {
		return nil, err
	}

	return pkgLayout, nil
}

// validateImageArchivesNoDuplicates ensures no image appears in multiple image archives
// and that images in image archives don't conflict with images in component.Images.
func validateImageArchivesNoDuplicates(components []v1alpha1.ZarfComponent) error {
	imageToArchive := make(map[string]string)

	for _, comp := range components {
		for _, archive := range comp.ImageArchives {
			for _, image := range archive.Images {
				refInfo, err := transform.ParseImageRef(image)
				if err != nil {
					return fmt.Errorf("failed to parse image ref %s in archive %s: %w", image, archive.Path, err)
				}

				if existingArchivePath, exists := imageToArchive[refInfo.Reference]; exists {
					// A user may want to represent the same tar twice across components if both components need the same image
					if existingArchivePath != archive.Path {
						return fmt.Errorf("image %s appears in multiple image archives: %s and %s", refInfo.Reference, existingArchivePath, archive.Path)
					}
				} else {
					imageToArchive[refInfo.Reference] = archive.Path
				}
			}
		}
	}

	for _, comp := range components {
		for _, image := range comp.Images {
			refInfo, err := transform.ParseImageRef(image)
			if err != nil {
				return fmt.Errorf("failed to parse image ref %s in component %s: %w", image, comp.Name, err)
			}
			if archivePath, exists := imageToArchive[refInfo.Reference]; exists {
				return fmt.Errorf("image %s from %s is also pulled by component %s", refInfo.Reference, archivePath, comp.Name)
			}
		}
	}

	return nil
}

func assemblePackageComponent(ctx context.Context, component v1alpha1.ZarfComponent, packagePath, buildPath, cachePath string, remoteOpts types.RemoteOptions) (err error) {
	tmpBuildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpBuildPath))
	}()
	compBuildPath := filepath.Join(tmpBuildPath, component.Name)
	err = os.MkdirAll(compBuildPath, 0o700)
	if err != nil {
		return err
	}

	onCreate := component.Actions.OnCreate
	if err := actions.Run(ctx, packagePath, onCreate.Defaults, onCreate.Before, nil, nil, template.StateAccess{}); err != nil {
		return fmt.Errorf("unable to run component before action: %w", err)
	}

	// If any helm charts are defined, process them.
	for _, chart := range component.Charts {
		paths := layout.ChartPaths{
			ChartsDir: filepath.Join(compBuildPath, string(layout.ChartsComponentDir)),
			ValuesDir: filepath.Join(compBuildPath, string(layout.ValuesComponentDir)),
		}
		err := PackageChart(ctx, chart, packagePath, paths, cachePath, remoteOpts)
		if err != nil {
			return err
		}
	}

	for filesIdx, file := range component.Files {
		rel := filepath.Join(string(layout.FilesComponentDir), layout.ComponentFileRelPath(filesIdx, file.Target))
		dst := filepath.Join(compBuildPath, rel)
		destinationDir := filepath.Dir(dst)

		if helpers.IsURL(file.Source) {
			if file.ExtractPath != "" {
				// get the compressedFileName from the source
				compressedFileName, err := helpers.ExtractBasePathFromURL(file.Source)
				if err != nil {
					return fmt.Errorf(lang.ErrFileNameExtract, file.Source, err)
				}
				tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
				if err != nil {
					return err
				}
				defer func() {
					err = errors.Join(err, os.RemoveAll(tmpDir))
				}()
				compressedFile := filepath.Join(tmpDir, compressedFileName)

				// If the file is an archive, download it to the componentPath.Temp
				if err := utils.DownloadToFile(ctx, file.Source, compressedFile); err != nil {
					return fmt.Errorf(lang.ErrDownloading, file.Source, err)
				}
				decompressOpts := archive.DecompressOpts{
					Files: []string{file.ExtractPath},
				}
				err = archive.Decompress(ctx, compressedFile, destinationDir, decompressOpts)
				if err != nil {
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, compressedFileName, err)
				}
			} else {
				if err := utils.DownloadToFile(ctx, file.Source, dst); err != nil {
					return fmt.Errorf(lang.ErrDownloading, file.Source, err)
				}
			}
		} else {
			src := file.Source
			if !filepath.IsAbs(file.Source) {
				src = filepath.Join(packagePath, file.Source)
			}
			if file.ExtractPath != "" {
				decompressOpts := archive.DecompressOpts{
					Files: []string{file.ExtractPath},
				}
				err = archive.Decompress(ctx, src, destinationDir, decompressOpts)
				if err != nil {
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, src, err)
				}
			} else {
				if err := helpers.CreatePathAndCopy(src, dst); err != nil {
					return fmt.Errorf("unable to copy file %s: %w", src, err)
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
				return fmt.Errorf("sha mismatch for %s: %w", file.Source, err)
			}
		}

		if file.Executable || helpers.IsDir(dst) {
			err := os.Chmod(dst, helpers.ReadWriteExecuteUser)
			if err != nil {
				return err
			}
		} else {
			err := os.Chmod(dst, helpers.ReadWriteUser)
			if err != nil {
				return err
			}
		}
	}

	for dataIdx, data := range component.DataInjections {
		rel := filepath.Join(string(layout.DataComponentDir), strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
		dst := filepath.Join(compBuildPath, rel)

		if helpers.IsURL(data.Source) {
			if err := utils.DownloadToFile(ctx, data.Source, dst); err != nil {
				return fmt.Errorf(lang.ErrDownloading, data.Source, err)
			}
		} else {
			src := data.Source
			if !filepath.IsAbs(data.Source) {
				src = filepath.Join(packagePath, data.Source)
			}
			if err := helpers.CreatePathAndCopy(src, dst); err != nil {
				return fmt.Errorf("unable to copy data injection %s: %w", data.Source, err)
			}
		}
	}

	// Iterate over all manifests.
	if len(component.Manifests) > 0 {
		err := os.MkdirAll(filepath.Join(compBuildPath, string(layout.ManifestsComponentDir)), 0o700)
		if err != nil {
			return err
		}
	}
	for _, manifest := range component.Manifests {
		err := PackageManifest(ctx, manifest, compBuildPath, packagePath)
		if err != nil {
			return err
		}
	}

	// Load all specified git repos.
	for _, url := range component.Repos {
		// Pull all the references if there is no `@` in the string.
		_, err := git.Clone(ctx, filepath.Join(compBuildPath, string(layout.RepoComponentDir)), url, false)
		if err != nil {
			return fmt.Errorf("unable to pull git repo %s: %w", url, err)
		}
	}

	if err := actions.Run(ctx, packagePath, onCreate.Defaults, onCreate.After, nil, nil, template.StateAccess{}); err != nil {
		return fmt.Errorf("unable to run component after action: %w", err)
	}

	// Write the tar component.
	entries, err := os.ReadDir(compBuildPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	tarPath := filepath.Join(buildPath, "components", fmt.Sprintf("%s.tar", component.Name))
	err = os.MkdirAll(filepath.Join(buildPath, "components"), 0o700)
	if err != nil {
		return err
	}
	err = createReproducibleTarballFromDir(compBuildPath, component.Name, tarPath, false)
	if err != nil {
		return err
	}
	return nil
}

// PackageManifest takes a Zarf manifest definition and packs it into a package layout
func PackageManifest(ctx context.Context, manifest v1alpha1.ZarfManifest, compBuildPath string, packagePath string) error {
	for fileIdx, path := range manifest.Files {
		rel := filepath.Join(string(layout.ManifestsComponentDir), layout.ManifestFileName(manifest.Name, fileIdx))
		dst := filepath.Join(compBuildPath, rel)

		// Copy manifests without any processing.
		if helpers.IsURL(path) {
			if err := utils.DownloadToFile(ctx, path, dst); err != nil {
				return fmt.Errorf(lang.ErrDownloading, path, err)
			}
		} else {
			src := path
			if !filepath.IsAbs(src) {
				src = filepath.Join(packagePath, src)
			}
			if err := helpers.CreatePathAndCopy(src, dst); err != nil {
				return fmt.Errorf("unable to copy manifest %s: %w", src, err)
			}
		}
	}

	for kustomizeIdx, path := range manifest.Kustomizations {
		// Generate manifests from kustomizations and place in the package.
		kname := layout.KustomizationFileName(manifest.Name, kustomizeIdx)
		rel := filepath.Join(string(layout.ManifestsComponentDir), kname)
		dst := filepath.Join(compBuildPath, rel)

		if !helpers.IsURL(path) && !filepath.IsAbs(path) {
			path = filepath.Join(packagePath, path)
		}
		if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory, manifest.EnableKustomizePlugins); err != nil {
			return fmt.Errorf("unable to build kustomization %s: %w", path, err)
		}
	}
	return nil
}

// PackageChart takes a Zarf Chart definition and packs it into a package layout
func PackageChart(ctx context.Context, chart v1alpha1.ZarfChart, packagePath string, paths layout.ChartPaths, cachePath string, remoteOpts types.RemoteOptions) error {
	if chart.LocalPath != "" && !filepath.IsAbs(chart.LocalPath) {
		chart.LocalPath = filepath.Join(packagePath, chart.LocalPath)
	}
	oldValuesFiles := chart.ValuesFiles
	valuesFiles := []string{}
	for _, v := range chart.ValuesFiles {
		if !helpers.IsURL(v) && !filepath.IsAbs(v) {
			v = filepath.Join(packagePath, v)
		}
		valuesFiles = append(valuesFiles, v)
	}
	chart.ValuesFiles = valuesFiles

	oldTemplatedValuesFiles := chart.TemplatedValuesFiles
	templatedValuesFiles := []string{}
	for _, v := range chart.TemplatedValuesFiles {
		if !helpers.IsURL(v) && !filepath.IsAbs(v) {
			v = filepath.Join(packagePath, v)
		}
		templatedValuesFiles = append(templatedValuesFiles, v)
	}
	chart.TemplatedValuesFiles = templatedValuesFiles

	if err := helm.PackageChart(ctx, chart, paths, cachePath, remoteOpts); err != nil {
		return err
	}
	chart.ValuesFiles = oldValuesFiles
	chart.TemplatedValuesFiles = oldTemplatedValuesFiles
	return nil
}

func assembleSkeletonComponent(ctx context.Context, component v1alpha1.ZarfComponent, packagePath, buildPath string) (err error) {
	tmpBuildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpBuildPath))
	}()
	compBuildPath := filepath.Join(tmpBuildPath, component.Name)
	err = os.MkdirAll(compBuildPath, 0o700)
	if err != nil {
		return err
	}

	for chartIdx, chart := range component.Charts {
		if chart.LocalPath != "" {
			rel := filepath.ToSlash(filepath.Join(string(layout.ChartsComponentDir), fmt.Sprintf("%s-%d", chart.Name, chartIdx)))
			dst := filepath.Join(compBuildPath, rel)

			file := chart.LocalPath
			if !filepath.IsAbs(file) {
				file = filepath.Join(packagePath, file)
			}
			if err := helpers.CreatePathAndCopy(file, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", file, err)
			}

			component.Charts[chartIdx].LocalPath = rel
		}

		for valuesIdx, path := range chart.ValuesFiles {
			if helpers.IsURL(path) {
				continue
			}

			rel := filepath.ToSlash(filepath.Join(string(layout.ValuesComponentDir), layout.ChartValuesFileName(chart.Name, chart.Version, valuesIdx)))
			component.Charts[chartIdx].ValuesFiles[valuesIdx] = rel

			if !filepath.IsAbs(path) {
				path = filepath.Join(packagePath, path)
			}
			if err := helpers.CreatePathAndCopy(path, filepath.Join(compBuildPath, rel)); err != nil {
				return fmt.Errorf("unable to copy chart values file %s: %w", path, err)
			}
		}

		nValuesFiles := len(chart.ValuesFiles)
		for valuesIdx, path := range chart.TemplatedValuesFiles {
			if helpers.IsURL(path) {
				continue
			}

			rel := filepath.ToSlash(filepath.Join(string(layout.ValuesComponentDir), layout.ChartValuesFileName(chart.Name, chart.Version, nValuesFiles+valuesIdx)))
			component.Charts[chartIdx].TemplatedValuesFiles[valuesIdx] = rel

			if !filepath.IsAbs(path) {
				path = filepath.Join(packagePath, path)
			}
			if err := helpers.CreatePathAndCopy(path, filepath.Join(compBuildPath, rel)); err != nil {
				return fmt.Errorf("unable to copy chart templated values file %s: %w", path, err)
			}
		}
	}

	for filesIdx, file := range component.Files {
		if helpers.IsURL(file.Source) {
			continue
		}

		rel := filepath.ToSlash(filepath.Join(string(layout.FilesComponentDir), layout.ComponentFileRelPath(filesIdx, file.Target)))
		dst := filepath.Join(compBuildPath, rel)
		destinationDir := filepath.Dir(dst)
		src := file.Source
		if !filepath.IsAbs(src) {
			src = filepath.Join(packagePath, src)
		}

		if file.ExtractPath != "" {
			decompressOpts := archive.DecompressOpts{
				Files: []string{file.ExtractPath},
			}
			err = archive.Decompress(ctx, src, destinationDir, decompressOpts)
			if err != nil {
				return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, src, err)
			}

			// Make sure dst reflects the actual file or directory.
			updatedExtractedFileOrDir := filepath.Join(destinationDir, file.ExtractPath)
			if updatedExtractedFileOrDir != dst {
				if err := os.Rename(updatedExtractedFileOrDir, dst); err != nil {
					return fmt.Errorf(lang.ErrWritingFile, dst, err)
				}
			}
		} else {
			if err := helpers.CreatePathAndCopy(src, dst); err != nil {
				return fmt.Errorf("unable to copy file %s: %w", src, err)
			}
		}

		// Change the source to the new relative source directory (any remote files will have been skipped above)
		component.Files[filesIdx].Source = rel

		// Remove the extractPath from a skeleton since it will already extract it
		component.Files[filesIdx].ExtractPath = ""

		// Abort packaging on invalid shasum (if one is specified).
		if file.Shasum != "" {
			if err := helpers.SHAsMatch(dst, file.Shasum); err != nil {
				return fmt.Errorf("sha mismatch for %s: %w", file.Source, err)
			}
		}

		if file.Executable || helpers.IsDir(dst) {
			err = os.Chmod(dst, helpers.ReadWriteExecuteUser)
			if err != nil {
				return err
			}
		} else {
			err = os.Chmod(dst, helpers.ReadWriteUser)
			if err != nil {
				return err
			}
		}
	}

	for dataIdx, data := range component.DataInjections {
		rel := filepath.ToSlash(filepath.Join(string(layout.DataComponentDir), strconv.Itoa(dataIdx), filepath.Base(data.Target.Path)))
		dst := filepath.Join(compBuildPath, rel)

		src := data.Source
		if !filepath.IsAbs(src) {
			src = filepath.Join(packagePath, src)
		}
		if err := helpers.CreatePathAndCopy(src, dst); err != nil {
			return fmt.Errorf("unable to copy data injection %s: %w", src, err)
		}

		component.DataInjections[dataIdx].Source = rel
	}
	// Iterate over all manifests.
	if len(component.Manifests) > 0 {
		err := os.MkdirAll(filepath.Join(compBuildPath, string(layout.ManifestsComponentDir)), 0o700)
		if err != nil {
			return err
		}
	}
	for manifestIdx, manifest := range component.Manifests {
		for fileIdx, path := range manifest.Files {
			rel := filepath.ToSlash(filepath.Join(string(layout.ManifestsComponentDir), layout.ManifestFileName(manifest.Name, fileIdx)))
			dst := filepath.Join(compBuildPath, rel)

			// Copy manifests without any processing.
			src := path
			if !filepath.IsAbs(src) {
				src = filepath.Join(packagePath, src)
			}
			if err := helpers.CreatePathAndCopy(src, dst); err != nil {
				return fmt.Errorf("unable to copy manifest %s: %w", src, err)
			}

			component.Manifests[manifestIdx].Files[fileIdx] = rel
		}

		for kustomizeIdx, path := range manifest.Kustomizations {
			// Generate manifests from kustomizations and place in the package.
			kname := layout.KustomizationFileName(manifest.Name, kustomizeIdx)
			rel := filepath.Join(string(layout.ManifestsComponentDir), kname)
			dst := filepath.Join(compBuildPath, rel)

			if !filepath.IsAbs(path) {
				path = filepath.Join(packagePath, path)
			}

			// Build() requires the path be present - otherwise will throw an error.
			if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory, manifest.EnableKustomizePlugins); err != nil {
				return fmt.Errorf("unable to build kustomization %s: %w", path, err)
			}
		}

		// remove kustomizations
		component.Manifests[manifestIdx].Kustomizations = nil
	}

	// Write the tar component.
	entries, err := os.ReadDir(compBuildPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	err = os.MkdirAll(filepath.Join(compBuildPath, "temp"), 0o700)
	if err != nil {
		return err
	}
	tarPath := filepath.Join(buildPath, "components", fmt.Sprintf("%s.tar", component.Name))
	err = os.MkdirAll(filepath.Join(buildPath, "components"), 0o700)
	if err != nil {
		return err
	}
	err = createReproducibleTarballFromDir(compBuildPath, component.Name, tarPath, true)
	if err != nil {
		return err
	}
	return nil
}

func recordPackageMetadata(definition *api.PackageDefinition, flavor string, registryOverrides []images.RegistryOverride, withBuildMachineInfo bool, buildPath, aggregateChecksum string) error {
	pkg := definition.AsV1alpha1()
	now := time.Now()
	buildData := api.BuildData{
		Architecture:      pkg.Metadata.Architecture,
		Timestamp:         now.Format(v1alpha1.BuildTimestampFormat),
		Version:           config.CLIVersion,
		Flavor:            flavor,
		ProvenanceFiles:   []string{layout.Checksums},
		AggregateChecksum: aggregateChecksum,
	}
	if withBuildMachineInfo {
		// Just use $USER env variable to avoid CGO issue.
		// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
		// Record the name of the user creating the package.
		if runtime.GOOS == "windows" {
			buildData.User = os.Getenv("USERNAME")
		} else {
			buildData.User = os.Getenv("USER")
		}

		// Record the hostname of the package creation terminal.
		//nolint: errcheck // The error here is ignored because the hostname is not critical to the package creation.
		hostname, _ := os.Hostname()
		buildData.Hostname = hostname
	}

	if pkg.IsInitConfig() && pkg.Metadata.Version == "" {
		definition.SetMetadataVersion(config.CLIVersion)
	}

	hasIndex := false
	if buildPath != "" {
		var err error
		hasIndex, err = layout.HasImageIndex(filepath.Join(buildPath, layout.ImagesDir))
		if err != nil {
			return fmt.Errorf("failed to inspect image layout: %w", err)
		}
	}
	buildData.VersionRequirements = collectVersionRequirements(pkg, hasIndex)

	// We lose the ordering for the user-provided registry overrides.
	overrides := make(map[string]string, len(registryOverrides))
	for i := range registryOverrides {
		overrides[registryOverrides[i].Source] = registryOverrides[i].Override
	}

	buildData.RegistryOverrides = overrides

	// Set signed to false by default; this is updated if signing occurs.
	signed := false
	buildData.Signed = &signed
	definition.SetBuildData(buildData)

	return nil
}

func collectVersionRequirements(pkg v1alpha1.ZarfPackage, hasIndex bool) []api.VersionRequirement {
	var reqs []api.VersionRequirement
	var hasImageArchives, hasTemplatedValuesFiles, hasVersionlessChart bool
	for _, comp := range pkg.Components {
		if !hasImageArchives && len(comp.ImageArchives) > 0 {
			hasImageArchives = true
		}
		for _, chart := range comp.Charts {
			if len(chart.TemplatedValuesFiles) > 0 {
				hasTemplatedValuesFiles = true
			}
			if chart.Version == "" {
				hasVersionlessChart = true
			}
		}
		if hasImageArchives && hasTemplatedValuesFiles && hasVersionlessChart {
			break
		}
	}
	if hasVersionlessChart {
		reqs = append(reqs, api.VersionRequirement{
			Version: "v0.65.0",
			Reason:  "This package contains a chart without a version, which is only supported on v0.65.0+",
		})
	}
	if hasImageArchives {
		reqs = append(reqs, api.VersionRequirement{
			Version: "v0.68.0",
			Reason:  "This package contains image archives which will only be recognized on v0.68.0+",
		})
	}
	if hasTemplatedValuesFiles {
		reqs = append(reqs, api.VersionRequirement{
			Version: "v0.78.0",
			Reason:  "This package uses templatedValuesFiles which require v0.78.0+",
		})
	}
	if hasIndex {
		reqs = append(reqs, api.VersionRequirement{
			Version: "v0.77.0",
			Reason:  "This package contains multi-platform images preserved by index digest, which require v0.77.0+",
		})
	}
	return reqs
}

// GetChecksum takes a directory then creates a sha256 check sum for each files in the
// directory recursively, then creates a global checksum for all the files together.
func GetChecksum(dirPath string) (string, string, error) {
	checksumData := []string{}
	err := filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		if rel == layout.ZarfYAML || rel == layout.Checksums {
			return nil
		}
		sum, err := helpers.GetSHA256OfFile(path)
		if err != nil {
			return err
		}
		checksumData = append(checksumData, fmt.Sprintf("%s %s", sum, filepath.ToSlash(rel)))
		return nil
	})
	if err != nil {
		return "", "", err
	}
	slices.Sort(checksumData)

	checksumContent := strings.Join(checksumData, "\n") + "\n"
	sha := sha256.Sum256([]byte(checksumContent))
	return checksumContent, hex.EncodeToString(sha[:]), nil
}

// createReproducibleTarballFromDir takes a directory then walks thru every file in that directory and adds it
// to a tar ball; the reproducible part comes from changing all the file user and group to 0, and settings the
// file creation, access, and mod time to midnight on January 1st 1970, UTC.
func createReproducibleTarballFromDir(dirPath string, dirPrefix string, tarballPath string, overrideMode bool) (err error) {
	tb, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %w", err)
	}
	defer func() {
		err = errors.Join(err, tb.Close())
	}()

	tw := tar.NewWriter(tb)
	defer func() {
		err = errors.Join(err, tw.Close())
	}()

	// Walk through the directory and process each file
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		link := ""
		if info.Mode().Type() == os.ModeSymlink {
			link, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("error reading symlink: %w", err)
			}
		}

		// Create a new header
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("error creating tar header: %w", err)
		}

		// Strip non-deterministic header data
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""

		// When run on windows the header mode will set all permission octals to the same value as the first octal.
		// A file created with 0o700 will return 0o777 when read back. This discrepancy causes differences between packages
		// created on Windows and Linux.
		// https://medium.com/@MichalPristas/go-and-file-perms-on-windows-3c944d55dd44
		// To mitigate this difference we zero all but the last permission octal when writing files to the tar. Making sure
		// that when unpackaged files from packages created on Windows and Linux will have the same permissions.
		// The &^ operator called AND NOT sets the bits to 0 in the left hand if the right hand bits are 1.
		// https://medium.com/learning-the-go-programming-language/bit-hacking-with-go-e0acee258827
		if overrideMode {
			header.Mode = header.Mode &^ 0o077
		}

		// Ensure the header's name is correctly set relative to the base directory
		name, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}
		name = filepath.Join(dirPrefix, name)
		name = filepath.ToSlash(name)
		header.Name = name

		// Write the header to the tarball
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing header: %w", err)
		}

		// If it's a file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("error opening file: %w", err)
			}
			defer func() {
				err = errors.Join(err, file.Close())
			}()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("error writing file to tarball: %w", err)
			}
		}

		return nil
	})
}

func mergeAndWriteValuesFile(ctx context.Context, files []string, packagePath, buildPath string) error {
	l := logger.From(ctx)

	if len(files) == 0 {
		return nil
	}

	// Build absolute paths for all values files
	valueFilePaths := make([]string, len(files))
	for i, file := range files {
		src := file
		if !filepath.IsAbs(src) {
			src = filepath.Join(packagePath, file)
		}
		// Validate src exists
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("unable to access values file %s: %w", src, err)
		}
		valueFilePaths[i] = src
	}

	// Parse and merge all values files
	vals, err := value.ParseFiles(ctx, valueFilePaths, value.ParseFilesOptions{})
	if err != nil {
		return fmt.Errorf("failed to parse values files: %w", err)
	}

	// Write merged values to YAML
	dst := filepath.Join(buildPath, layout.ValuesYAML)
	l.Debug("writing merged values file", "dst", dst, "fileCount", len(files))
	if err := utils.WriteYaml(dst, vals, helpers.ReadWriteUser); err != nil {
		return fmt.Errorf("failed to write merged values file: %w", err)
	}

	return nil
}

// mergeAndWriteValuesSchema merges imported child schemas with the parent schema (parent wins)
// and writes the result to buildPath/values.schema.json. If only a parent schema exists with
// no imports, it is validated and copied as-is. If only child schemas exist, they are merged
// and written. If neither exists, the function is a no-op.
//
// Schemas containing "$ref" pointers are rejected in all cases because references may point
// to files unavailable after assembly.
func mergeAndWriteValuesSchema(ctx context.Context, parentSchema string, importedSchemas []string, packagePath, buildPath string) error {
	l := logger.From(ctx)

	if parentSchema == "" && len(importedSchemas) == 0 {
		return nil
	}

	// No child schemas — check for $ref, validate, then copy the parent schema file verbatim.
	if len(importedSchemas) == 0 {
		_, src, err := value.LoadValidatedSchema(packagePath, parentSchema)
		if err != nil {
			return err
		}
		dst := filepath.Join(buildPath, layout.ValuesSchema)
		l.Debug("copying values schema file", "src", src, "dst", dst)
		if err := helpers.CreatePathAndCopy(src, dst); err != nil {
			return fmt.Errorf("failed to copy values schema file %s: %w", parentSchema, err)
		}
		return os.Chmod(dst, helpers.ReadWriteUser)
	}

	l.Debug("merging values schemas", "parent", parentSchema, "imported", len(importedSchemas))

	// Merge child schemas left-to-right; among children the earlier one wins.
	merged, err := value.MergeSchemaFiles(parentSchema, importedSchemas, packagePath)
	if err != nil {
		return fmt.Errorf("merging schemas: %w", err)
	}

	if err := value.ValidateSchemaDocument(merged); err != nil {
		return fmt.Errorf("merged values schema is invalid: %w", err)
	}

	dst := filepath.Join(buildPath, layout.ValuesSchema)
	l.Debug("writing merged values schema", "dst", dst)
	b, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merged values schema: %w", err)
	}
	if err := os.WriteFile(dst, b, helpers.ReadWriteUser); err != nil {
		return fmt.Errorf("failed to write merged values schema: %w", err)
	}
	return nil
}

func createDocumentationTar(pkg v1alpha1.ZarfPackage, packagePath, buildPath string) (err error) {
	if len(pkg.Documentation) == 0 {
		return nil
	}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fmt.Errorf("failed to create temp directory for documentation: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpDir))
	}()

	// Get the mapping of keys to their final filenames (with deduplication logic)
	fileNames := layout.GetDocumentationFileNames(pkg.Documentation)

	for key, file := range pkg.Documentation {
		src := file
		if !filepath.IsAbs(src) {
			src = filepath.Join(packagePath, file)
		}

		docFilename := fileNames[key]
		dst := filepath.Join(tmpDir, docFilename)

		if err := helpers.CreatePathAndCopy(src, dst); err != nil {
			return fmt.Errorf("failed to copy documentation file %s: %w", src, err)
		}

		if err := os.Chmod(dst, helpers.ReadWriteUser); err != nil {
			return fmt.Errorf("failed to set permissions on documentation file %s: %w", dst, err)
		}
	}

	tarPath := filepath.Join(buildPath, layout.DocumentationTar)
	if err := createReproducibleTarballFromDir(tmpDir, "", tarPath, true); err != nil {
		return fmt.Errorf("failed to create documentation tarball: %w", err)
	}

	return nil
}
