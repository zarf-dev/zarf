// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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
}

// AssemblePackage takes a package definition and returns a package layout with all the resources collected
func AssemblePackage(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, opts AssembleOptions) (*PackageLayout, error) {
	l := logger.From(ctx)
	l.Info("assembling package", "path", packagePath)

	if err := validateImageArchivesNoDuplicates(pkg.Components); err != nil {
		return nil, err
	}

	if opts.DifferentialPackage.Metadata.Name != "" {
		l.Debug("creating differential package", "differential", opts.DifferentialPackage)
		allIncludedImagesMap := map[string]bool{}
		allIncludedReposMap := map[string]bool{}
		for _, component := range opts.DifferentialPackage.Components {
			for _, image := range component.Images {
				allIncludedImagesMap[image] = true
			}
			for _, repo := range component.Repos {
				allIncludedReposMap[repo] = true
			}
		}

		pkg.Build.Differential = true
		pkg.Build.DifferentialPackageVersion = opts.DifferentialPackage.Metadata.Version

		versionsMatch := opts.DifferentialPackage.Metadata.Version == pkg.Metadata.Version
		if versionsMatch {
			return nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		noVersionSet := opts.DifferentialPackage.Metadata.Version == "" || pkg.Metadata.Version == ""
		if noVersionSet {
			return nil, errors.New(lang.PkgCreateErrDifferentialNoVersion)
		}
		filter := filters.ByDifferentialData(allIncludedImagesMap, allIncludedReposMap)
		var err error
		pkg.Components, err = filter.Apply(pkg)
		if err != nil {
			return nil, err
		}
	}

	buildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	for _, component := range pkg.Components {
		err := assemblePackageComponent(ctx, component, packagePath, buildPath, opts.CachePath)
		if err != nil {
			return nil, err
		}
	}

	componentImages := []transform.Image{}
	manifests := []images.ImageWithManifest{}
	for _, component := range pkg.Components {
		for _, imageArchive := range component.ImageArchives {
			if !filepath.IsAbs(imageArchive.Path) {
				imageArchive.Path = filepath.Join(packagePath, imageArchive.Path)
			}

			archiveImageManifests, err := images.Unpack(ctx, imageArchive, filepath.Join(buildPath, ImagesDir), pkg.Metadata.Architecture)
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
			CacheDirectory:        filepath.Join(opts.CachePath, ImagesDir),
			PlainHTTP:             config.CommonOptions.PlainHTTP,
			InsecureSkipTLSVerify: config.CommonOptions.InsecureSkipTLSVerify,
		}
		imageManifests, err := images.Pull(ctx, componentImages, filepath.Join(buildPath, ImagesDir), pullOpts)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, imageManifests...)
	}

	for _, manifest := range manifests {
		ok := images.OnlyHasImageLayers(manifest.Manifest)
		if ok {
			sbomImageList = append(sbomImageList, manifest.Image)
		}

		// Sort images index to make build reproducible.
		err = utils.SortImagesIndex(filepath.Join(buildPath, ImagesDir))
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

	// Copy schema file if specified
	if pkg.Values.Schema != "" {
		if err = copyValuesSchema(ctx, pkg.Values.Schema, packagePath, buildPath); err != nil {
			return nil, err
		}
	}

	if err = createDocumentationTar(pkg, packagePath, buildPath); err != nil {
		return nil, err
	}

	checksumContent, checksumSha, err := getChecksum(buildPath)
	if err != nil {
		return nil, err
	}
	checksumPath := filepath.Join(buildPath, Checksums)
	err = os.WriteFile(checksumPath, []byte(checksumContent), helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}
	pkg.Metadata.AggregateChecksum = checksumSha

	pkg = recordPackageMetadata(pkg, opts.Flavor, opts.RegistryOverrides, opts.WithBuildMachineInfo)

	b, err := goyaml.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(buildPath, ZarfYAML), b, helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}

	// skip verification on package creation
	pkgLayout, err := LoadFromDir(ctx, buildPath, PackageLayoutOptions{VerificationStrategy: VerifyNever})
	if err != nil {
		return nil, err
	}

	// Sign the package with the provided options
	signOpts := utils.DefaultSignBlobOptions()
	signOpts.KeyRef = opts.SigningKeyPath
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
func AssembleSkeleton(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, opts AssembleSkeletonOptions) (*PackageLayout, error) {
	pkg.Metadata.Architecture = v1alpha1.SkeletonArch

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

	checksumContent, checksumSha, err := getChecksum(buildPath)
	if err != nil {
		return nil, err
	}
	checksumPath := filepath.Join(buildPath, Checksums)
	err = os.WriteFile(checksumPath, []byte(checksumContent), helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}
	pkg.Metadata.AggregateChecksum = checksumSha

	pkg = recordPackageMetadata(pkg, opts.Flavor, nil, opts.WithBuildMachineInfo)

	b, err := goyaml.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(buildPath, ZarfYAML), b, helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}

	layoutOpts := PackageLayoutOptions{
		VerificationStrategy: VerifyNever,
		IsPartial:            false,
	}
	pkgLayout, err := LoadFromDir(ctx, buildPath, layoutOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to load skeleton: %w", err)
	}

	// Sign the package with the provided options
	signOpts := utils.DefaultSignBlobOptions()
	signOpts.KeyRef = opts.SigningKeyPath
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

func assemblePackageComponent(ctx context.Context, component v1alpha1.ZarfComponent, packagePath, buildPath, cachePath string) (err error) {
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
	if err := actions.Run(ctx, packagePath, onCreate.Defaults, onCreate.Before, nil, nil); err != nil {
		return fmt.Errorf("unable to run component before action: %w", err)
	}

	// If any helm charts are defined, process them.
	for _, chart := range component.Charts {
		chartPath := filepath.Join(compBuildPath, string(ChartsComponentDir))
		valuesFilePath := filepath.Join(compBuildPath, string(ValuesComponentDir))
		err := PackageChart(ctx, chart, packagePath, chartPath, valuesFilePath, cachePath)
		if err != nil {
			return err
		}
	}

	for filesIdx, file := range component.Files {
		rel := filepath.Join(string(FilesComponentDir), strconv.Itoa(filesIdx), filepath.Base(file.Target))
		dst := filepath.Join(compBuildPath, rel)
		destinationDir := filepath.Dir(dst)

		if helpers.IsURL(file.Source) {
			if file.ExtractPath != "" {
				// get the compressedFileName from the source
				compressedFileName, err := helpers.ExtractBasePathFromURL(file.Source)
				if err != nil {
					return fmt.Errorf(lang.ErrFileNameExtract, file.Source, err.Error())
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
					return fmt.Errorf(lang.ErrDownloading, file.Source, err.Error())
				}
				decompressOpts := archive.DecompressOpts{
					Files: []string{file.ExtractPath},
				}
				err = archive.Decompress(ctx, compressedFile, destinationDir, decompressOpts)
				if err != nil {
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, compressedFileName, err.Error())
				}
			} else {
				if err := utils.DownloadToFile(ctx, file.Source, dst); err != nil {
					return fmt.Errorf(lang.ErrDownloading, file.Source, err.Error())
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
					return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, src, err.Error())
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
		rel := filepath.Join(string(DataComponentDir), strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
		dst := filepath.Join(compBuildPath, rel)

		if helpers.IsURL(data.Source) {
			if err := utils.DownloadToFile(ctx, data.Source, dst); err != nil {
				return fmt.Errorf(lang.ErrDownloading, data.Source, err.Error())
			}
		} else {
			src := data.Source
			if !filepath.IsAbs(data.Source) {
				src = filepath.Join(packagePath, data.Source)
			}
			if err := helpers.CreatePathAndCopy(src, dst); err != nil {
				return fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
			}
		}
	}

	// Iterate over all manifests.
	if len(component.Manifests) > 0 {
		err := os.MkdirAll(filepath.Join(compBuildPath, string(ManifestsComponentDir)), 0o700)
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
		_, err := git.Clone(ctx, filepath.Join(compBuildPath, string(RepoComponentDir)), url, false)
		if err != nil {
			return fmt.Errorf("unable to pull git repo %s: %w", url, err)
		}
	}

	if err := actions.Run(ctx, packagePath, onCreate.Defaults, onCreate.After, nil, nil); err != nil {
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
		rel := filepath.Join(string(ManifestsComponentDir), fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
		dst := filepath.Join(compBuildPath, rel)

		// Copy manifests without any processing.
		if helpers.IsURL(path) {
			if err := utils.DownloadToFile(ctx, path, dst); err != nil {
				return fmt.Errorf(lang.ErrDownloading, path, err.Error())
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
		kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
		rel := filepath.Join(string(ManifestsComponentDir), kname)
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
func PackageChart(ctx context.Context, chart v1alpha1.ZarfChart, packagePath, chartPath, valuesFilePath, cachePath string) error {
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
	if err := helm.PackageChart(ctx, chart, chartPath, valuesFilePath, cachePath); err != nil {
		return err
	}
	chart.ValuesFiles = oldValuesFiles
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
			rel := filepath.Join(string(ChartsComponentDir), fmt.Sprintf("%s-%d", chart.Name, chartIdx))
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

			rel := fmt.Sprintf("%s-%d", helm.StandardName(string(ValuesComponentDir), chart), valuesIdx)
			component.Charts[chartIdx].ValuesFiles[valuesIdx] = rel

			if !filepath.IsAbs(path) {
				path = filepath.Join(packagePath, path)
			}
			if err := helpers.CreatePathAndCopy(path, filepath.Join(compBuildPath, rel)); err != nil {
				return fmt.Errorf("unable to copy chart values file %s: %w", path, err)
			}
		}
	}

	for filesIdx, file := range component.Files {
		if helpers.IsURL(file.Source) {
			continue
		}

		rel := filepath.Join(string(FilesComponentDir), strconv.Itoa(filesIdx), filepath.Base(file.Target))
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
				return fmt.Errorf(lang.ErrFileExtract, file.ExtractPath, src, err.Error())
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
		rel := filepath.Join(string(DataComponentDir), strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
		dst := filepath.Join(compBuildPath, rel)

		src := data.Source
		if !filepath.IsAbs(src) {
			src = filepath.Join(packagePath, src)
		}
		if err := helpers.CreatePathAndCopy(src, dst); err != nil {
			return fmt.Errorf("unable to copy data injection %s: %s", src, err.Error())
		}

		component.DataInjections[dataIdx].Source = rel
	}
	// Iterate over all manifests.
	if len(component.Manifests) > 0 {
		err := os.MkdirAll(filepath.Join(compBuildPath, string(ManifestsComponentDir)), 0o700)
		if err != nil {
			return err
		}
	}
	for manifestIdx, manifest := range component.Manifests {
		for fileIdx, path := range manifest.Files {
			rel := filepath.Join(string(ManifestsComponentDir), fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
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
			kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx)
			rel := filepath.Join(string(ManifestsComponentDir), kname)
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

func recordPackageMetadata(pkg v1alpha1.ZarfPackage, flavor string, registryOverrides []images.RegistryOverride, withBuildMachineInfo bool) v1alpha1.ZarfPackage {
	now := time.Now()
	if withBuildMachineInfo {
		// Just use $USER env variable to avoid CGO issue.
		// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
		// Record the name of the user creating the package.
		if runtime.GOOS == "windows" {
			pkg.Build.User = os.Getenv("USERNAME")
		} else {
			pkg.Build.User = os.Getenv("USER")
		}

		// Record the hostname of the package creation terminal.
		//nolint: errcheck // The error here is ignored because the hostname is not critical to the package creation.
		hostname, _ := os.Hostname()
		pkg.Build.Terminal = hostname
	}

	if pkg.IsInitConfig() && pkg.Metadata.Version == "" {
		pkg.Metadata.Version = config.CLIVersion
	}

	pkg.Build.Architecture = pkg.Metadata.Architecture

	// Record the Zarf Version the CLI was built with.
	pkg.Build.Version = config.CLIVersion

	// Record the time of package creation.
	pkg.Build.Timestamp = now.Format(v1alpha1.BuildTimestampFormat)

	// Record the flavor of Zarf used to build this package (if any).
	pkg.Build.Flavor = flavor

	var versionRequirements []v1alpha1.VersionRequirement
	for _, comp := range pkg.Components {
		if len(comp.ImageArchives) > 0 {
			versionRequirements = append(versionRequirements, v1alpha1.VersionRequirement{
				Version: "v0.68.0",
				Reason:  "This package contains image archives which will only be recognized on v0.68.0+",
			})
			break
		}
	}
	pkg.Build.VersionRequirements = versionRequirements

	// We lose the ordering for the user-provided registry overrides.
	overrides := make(map[string]string, len(registryOverrides))
	for i := range registryOverrides {
		overrides[registryOverrides[i].Source] = registryOverrides[i].Override
	}

	pkg.Build.RegistryOverrides = overrides

	// set signed to false by default - this is updated if signing occurs.
	signed := false
	pkg.Build.Signed = &signed

	return pkg
}

func getChecksum(dirPath string) (string, string, error) {
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
		if rel == ZarfYAML || rel == Checksums {
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

func createReproducibleTarballFromDir(dirPath, dirPrefix, tarballPath string, overrideMode bool) (err error) {
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
	dst := filepath.Join(buildPath, ValuesYAML)
	l.Debug("writing merged values file", "dst", dst, "fileCount", len(files))
	if err := utils.WriteYaml(dst, vals, helpers.ReadWriteUser); err != nil {
		return fmt.Errorf("failed to write merged values file: %w", err)
	}

	return nil
}

// copyValuesSchema validates and copies a values schema file to the build directory.
// It validates the schema is valid JSON Schema, checks for path traversal, and copies
// the file to the package root.
func copyValuesSchema(ctx context.Context, schema, packagePath, buildPath string) error {
	l := logger.From(ctx)
	l.Debug("copying values schema file to package", "schema", schema)

	// Resolve the schema source path from package root
	schemaSrc := schema
	if !filepath.IsAbs(schemaSrc) {
		schemaSrc = filepath.Join(packagePath, schema)
	}

	// Validate the schema is valid JSON Schema
	if err := value.ValidateSchemaFile(schemaSrc); err != nil {
		return fmt.Errorf("values schema validation failed: %w", err)
	}

	// Copy schema file to package root
	schemaDst := filepath.Join(buildPath, ValuesSchema)
	l.Debug("copying values schema file", "src", schemaSrc, "dst", schemaDst)
	if err := helpers.CreatePathAndCopy(schemaSrc, schemaDst); err != nil {
		return fmt.Errorf("failed to copy values schema file %s: %w", schemaSrc, err)
	}

	// Set appropriate file permissions
	if err := os.Chmod(schemaDst, helpers.ReadWriteUser); err != nil {
		return fmt.Errorf("failed to set permissions on values schema file %s: %w", schemaDst, err)
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
	fileNames := GetDocumentationFileNames(pkg.Documentation)

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

	tarPath := filepath.Join(buildPath, DocumentationTar)
	if err := createReproducibleTarballFromDir(tmpDir, "", tarPath, true); err != nil {
		return fmt.Errorf("failed to create documentation tarball: %w", err)
	}

	return nil
}
