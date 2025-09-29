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
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	actions2 "github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// AssembleOptions are the options for creating a package from a package object
type AssembleOptions struct {
	// Flavor causes the package to only include components with a matching `.components[x].only.flavor` or no flavor `.components[x].only.flavor` specified
	Flavor string
	// RegistryOverrides overrides the basepath of an OCI image with a path to a different registry
	RegistryOverrides  map[string]string
	SigningKeyPath     string
	SigningKeyPassword string
	SkipSBOM           bool
	// When DifferentialPackage is set the zarf package created only includes images and repos not in the differential package
	DifferentialPackage v1alpha1.ZarfPackage
	OCIConcurrency      int
	// CachePath is the path to the Zarf cache, used to cache images
	CachePath string
}

// AssemblePackage takes a package definition and returns a package layout with all the resources collected
func AssemblePackage(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, opts AssembleOptions) (*PackageLayout, error) {
	l := logger.From(ctx)
	l.Info("assembling package", "path", packagePath)

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
		err := assemblePackageComponent(ctx, component, packagePath, buildPath)
		if err != nil {
			return nil, err
		}
	}

	componentImages := []transform.Image{}
	for _, component := range pkg.Components {
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
		pullCfg := images.PullConfig{
			OCIConcurrency:        opts.OCIConcurrency,
			DestinationDirectory:  filepath.Join(buildPath, ImagesDir),
			ImageList:             componentImages,
			Arch:                  pkg.Metadata.Architecture,
			RegistryOverrides:     opts.RegistryOverrides,
			CacheDirectory:        filepath.Join(opts.CachePath, ImagesDir),
			PlainHTTP:             config.CommonOptions.PlainHTTP,
			InsecureSkipTLSVerify: config.CommonOptions.InsecureSkipTLSVerify,
		}
		manifests, err := images.Pull(ctx, pullCfg)
		if err != nil {
			return nil, err
		}
		for image, manifest := range manifests {
			ok := images.OnlyHasImageLayers(manifest)
			if ok {
				sbomImageList = append(sbomImageList, image)
			}
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

	pkg = recordPackageMetadata(pkg, opts.Flavor, opts.RegistryOverrides)

	b, err := goyaml.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(buildPath, ZarfYAML), b, helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}

	err = signPackage(buildPath, opts.SigningKeyPath, opts.SigningKeyPassword)
	if err != nil {
		return nil, err
	}

	pkgLayout, err := LoadFromDir(ctx, buildPath, PackageLayoutOptions{SkipSignatureValidation: true})
	if err != nil {
		return nil, err
	}

	return pkgLayout, nil
}

// AssembleSkeletonOptions are the options for creating a skeleton package
type AssembleSkeletonOptions struct {
	SigningKeyPath     string
	SigningKeyPassword string
	Flavor             string
}

// AssembleSkeleton creates a skeleton package and returns the path to the created package.
func AssembleSkeleton(ctx context.Context, pkg v1alpha1.ZarfPackage, packagePath string, opts AssembleSkeletonOptions) (*PackageLayout, error) {
	pkg.Metadata.Architecture = v1alpha1.SkeletonArch

	buildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
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

	pkg = recordPackageMetadata(pkg, opts.Flavor, nil)

	b, err := goyaml.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(buildPath, ZarfYAML), b, helpers.ReadWriteUser)
	if err != nil {
		return nil, err
	}

	err = signPackage(buildPath, opts.SigningKeyPath, opts.SigningKeyPassword)
	if err != nil {
		return nil, err
	}

	layoutOpts := PackageLayoutOptions{
		SkipSignatureValidation: true,
		IsPartial:               false,
	}
	pkgLayout, err := LoadFromDir(ctx, buildPath, layoutOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to load skeleton: %w", err)
	}

	return pkgLayout, nil
}

func assemblePackageComponent(ctx context.Context, component v1alpha1.ZarfComponent, packagePath, buildPath string) (err error) {
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
	if err := actions2.Run(ctx, packagePath, onCreate.Defaults, onCreate.Before, nil); err != nil {
		return fmt.Errorf("unable to run component before action: %w", err)
	}

	// If any helm charts are defined, process them.
	for _, chart := range component.Charts {
		chartPath := filepath.Join(compBuildPath, string(ChartsComponentDir))
		valuesFilePath := filepath.Join(compBuildPath, string(ValuesComponentDir))
		err := PackageChart(ctx, chart, packagePath, chartPath, valuesFilePath)
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
				return err
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

	if err := actions2.Run(ctx, packagePath, onCreate.Defaults, onCreate.After, nil); err != nil {
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
		if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
			return fmt.Errorf("unable to build kustomization %s: %w", path, err)
		}
	}
	return nil
}

// PackageChart takes a Zarf Chart definition and packs it into a package layout
func PackageChart(ctx context.Context, chart v1alpha1.ZarfChart, packagePath string, chartPath string, valuesFilePath string) error {
	if chart.LocalPath != "" && !filepath.IsAbs(chart.LocalPath) {
		chart.LocalPath = filepath.Join(packagePath, chart.LocalPath)
	}
	oldValuesFiles := chart.ValuesFiles
	valuesFiles := []string{}
	for _, v := range chart.ValuesFiles {
		if !filepath.IsAbs(v) {
			v = filepath.Join(packagePath, v)
		}
		valuesFiles = append(valuesFiles, v)
	}
	chart.ValuesFiles = valuesFiles
	if err := helm.PackageChart(ctx, chart, chartPath, valuesFilePath); err != nil {
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
				return err
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
			if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
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

func recordPackageMetadata(pkg v1alpha1.ZarfPackage, flavor string, registryOverrides map[string]string) v1alpha1.ZarfPackage {
	now := time.Now()
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

	pkg.Build.RegistryOverrides = registryOverrides

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

func signPackage(dirPath, signingKeyPath, signingKeyPassword string) error {
	if signingKeyPath == "" {
		return nil
	}
	passFunc := func(_ bool) ([]byte, error) {
		return []byte(signingKeyPassword), nil
	}
	keyOpts := options.KeyOpts{
		KeyRef:   signingKeyPath,
		PassFunc: passFunc,
	}
	rootOpts := &options.RootOptions{
		Verbose: false,
		Timeout: options.DefaultTimeout,
	}
	_, err := sign.SignBlobCmd(
		rootOpts,
		keyOpts,
		filepath.Join(dirPath, ZarfYAML),
		true,
		filepath.Join(dirPath, Signature),
		"",
		false)
	if err != nil {
		return err
	}
	return nil
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
