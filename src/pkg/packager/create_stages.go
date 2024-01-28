// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mholt/archiver/v3"
)

func (p *Packager) cdToBaseDir(base string, cwd string) error {
	if err := os.Chdir(base); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", base, err)
	}
	message.Note(fmt.Sprintf("Using build directory %s", base))

	// differentials are relative to the current working directory
	if p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath != "" {
		p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath = filepath.Join(cwd, p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath)
	}
	return nil
}

func (p *Packager) load() error {
	if err := p.readZarfYAML(layout.ZarfYAML); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml file: %s", err.Error())
	}
	if p.isInitConfig() {
		p.cfg.Pkg.Metadata.Version = config.CLIVersion
	}

	// Compose components into a single zarf.yaml file
	if err := p.composeComponents(); err != nil {
		return err
	}

	if p.cfg.CreateOpts.IsSkeleton {
		return nil
	}

	// After components are composed, template the active package.
	if err := p.fillActiveTemplate(); err != nil {
		return fmt.Errorf("unable to fill values in template: %s", err.Error())
	}

	// After templates are filled process any create extensions
	if err := p.processExtensions(); err != nil {
		return err
	}

	// After we have a full zarf.yaml remove unnecessary repos and images if we are building a differential package
	if p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath != "" {
		// Load the images and repos from the 'reference' package
		if err := p.loadDifferentialData(); err != nil {
			return err
		}
		// Verify the package version of the package we're using as a 'reference' for the differential build is different than the package we're building
		// If the package versions are the same return an error
		if p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion == p.cfg.Pkg.Metadata.Version {
			return errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		if p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion == "" || p.cfg.Pkg.Metadata.Version == "" {
			return fmt.Errorf("unable to build differential package when either the differential package version or the referenced package version is not set")
		}

		// Handle any potential differential images/repos before going forward
		if err := p.removeCopiesFromDifferentialPackage(); err != nil {
			return err
		}
	}

	return nil
}

func (p *Packager) assemble() error {
	componentSBOMs := map[string]*layout.ComponentSBOM{}
	var imageList []transform.Image
	for idx, component := range p.cfg.Pkg.Components {
		onCreate := component.Actions.OnCreate
		onFailure := func() {
			if err := p.runActions(onCreate.Defaults, onCreate.OnFailure, nil); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}
		if err := p.addComponent(idx, component); err != nil {
			onFailure()
			return fmt.Errorf("unable to add component %q: %w", component.Name, err)
		}

		if err := p.runActions(onCreate.Defaults, onCreate.OnSuccess, nil); err != nil {
			onFailure()
			return fmt.Errorf("unable to run component success action: %w", err)
		}

		if !p.cfg.CreateOpts.SkipSBOM {
			componentSBOM, err := p.getFilesToSBOM(component)
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

		p.layout = p.layout.AddImages()

		var pulled []images.ImgInfo
		var err error

		doPull := func() error {
			imgConfig := images.ImageConfig{
				ImagesPath:        p.layout.Images.Base,
				ImageList:         imageList,
				Insecure:          config.CommonOptions.Insecure,
				Architectures:     []string{p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture},
				RegistryOverrides: p.cfg.CreateOpts.RegistryOverrides,
			}

			pulled, err = imgConfig.PullAll()
			return err
		}

		if err := helpers.Retry(doPull, 3, 5*time.Second, message.Warnf); err != nil {
			return fmt.Errorf("unable to pull images after 3 attempts: %w", err)
		}

		for _, imgInfo := range pulled {
			if err := p.layout.Images.AddV1Image(imgInfo.Img); err != nil {
				return err
			}
			if imgInfo.HasImageLayers {
				sbomImageList = append(sbomImageList, imgInfo.RefInfo)
			}
		}
	}

	// Ignore SBOM creation if the flag is set.
	if p.cfg.CreateOpts.SkipSBOM {
		message.Debug("Skipping image SBOM processing per --skip-sbom flag")
	} else {
		p.layout = p.layout.AddSBOMs()
		if err := sbom.Catalog(componentSBOMs, sbomImageList, p.layout); err != nil {
			return fmt.Errorf("unable to create an SBOM catalog for the package: %w", err)
		}
	}

	return nil
}

func (p *Packager) assembleSkeleton() error {
	if err := p.skeletonizeExtensions(); err != nil {
		return err
	}
	for _, warning := range p.warnings {
		message.Warn(warning)
	}
	for idx, component := range p.cfg.Pkg.Components {
		if err := p.addComponent(idx, component); err != nil {
			return err
		}

		if err := p.layout.Components.Archive(component, false); err != nil {
			return err
		}
	}
	checksumChecksum, err := p.generatePackageChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for skeleton package: %w", err)
	}
	p.cfg.Pkg.Metadata.AggregateChecksum = checksumChecksum

	return p.writeYaml()
}

// output assumes it is running from cwd, not the build directory
func (p *Packager) output() error {
	// Process the component directories into compressed tarballs
	// NOTE: This is purposefully being done after the SBOM cataloging
	for _, component := range p.cfg.Pkg.Components {
		// Make the component a tar archive
		if err := p.layout.Components.Archive(component, true); err != nil {
			return fmt.Errorf("unable to archive component: %s", err.Error())
		}
	}

	// Calculate all the checksums
	checksumChecksum, err := p.generatePackageChecksums()
	if err != nil {
		return fmt.Errorf("unable to generate checksums for the package: %w", err)
	}
	p.cfg.Pkg.Metadata.AggregateChecksum = checksumChecksum

	// Save the transformed config.
	if err := p.writeYaml(); err != nil {
		return fmt.Errorf("unable to write zarf.yaml: %w", err)
	}

	// Sign the config file if a key was provided
	if p.cfg.CreateOpts.SigningKeyPath != "" {
		if err := p.signPackage(p.cfg.CreateOpts.SigningKeyPath, p.cfg.CreateOpts.SigningKeyPassword); err != nil {
			return err
		}
	}

	// Create a remote ref + client for the package (if output is OCI)
	// then publish the package to the remote.
	if helpers.IsOCIURL(p.cfg.CreateOpts.Output) {
		ref, err := oci.ReferenceFromMetadata(p.cfg.CreateOpts.Output, &p.cfg.Pkg.Metadata, &p.cfg.Pkg.Build)
		if err != nil {
			return err
		}
		remote, err := oci.NewOrasRemote(ref, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			return err
		}

		err = remote.PublishPackage(&p.cfg.Pkg, p.layout, config.CommonOptions.OCIConcurrency)
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
		packageName := filepath.Join(p.cfg.CreateOpts.Output, p.GetPackageName())

		// Try to remove the package if it already exists.
		_ = os.Remove(packageName)

		// Create the package tarball.
		if err := p.archivePackage(packageName); err != nil {
			return fmt.Errorf("unable to archive package: %w", err)
		}
	}

	// Output the SBOM files into a directory if specified.
	if p.cfg.CreateOpts.ViewSBOM || p.cfg.CreateOpts.SBOMOutputDir != "" {
		outputSBOM := p.cfg.CreateOpts.SBOMOutputDir
		var sbomDir string
		if err := p.layout.SBOMs.Unarchive(); err != nil {
			return fmt.Errorf("unable to unarchive SBOMs: %w", err)
		}
		sbomDir = p.layout.SBOMs.Path

		if outputSBOM != "" {
			out, err := sbom.OutputSBOMFiles(sbomDir, outputSBOM, p.cfg.Pkg.Metadata.Name)
			if err != nil {
				return err
			}
			sbomDir = out
		}

		if p.cfg.CreateOpts.ViewSBOM {
			sbom.ViewSBOMFiles(sbomDir)
		}
	}
	return nil
}

func (p *Packager) getFilesToSBOM(component types.ZarfComponent) (*layout.ComponentSBOM, error) {
	componentPaths, err := p.layout.Components.Create(component)
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

func (p *Packager) addComponent(index int, component types.ZarfComponent) error {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	isSkeleton := p.cfg.CreateOpts.IsSkeleton

	componentPaths, err := p.layout.Components.Create(component)
	if err != nil {
		return err
	}

	if isSkeleton && component.DeprecatedCosignKeyPath != "" {
		dst := filepath.Join(componentPaths.Base, "cosign.pub")
		err := utils.CreatePathAndCopy(component.DeprecatedCosignKeyPath, dst)
		if err != nil {
			return err
		}
		p.cfg.Pkg.Components[index].DeprecatedCosignKeyPath = "cosign.pub"
	}

	// TODO: (@WSTARR) Shim the skeleton component's create action dirs to be empty.  This prevents actions from failing by cd'ing into directories that will be flattened.
	if isSkeleton {
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
	}

	onCreate := component.Actions.OnCreate
	if !isSkeleton {
		if err := p.runActions(onCreate.Defaults, onCreate.Before, nil); err != nil {
			return fmt.Errorf("unable to run component before action: %w", err)
		}
	}

	// If any helm charts are defined, process them.
	for chartIdx, chart := range component.Charts {

		helmCfg := helm.New(chart, componentPaths.Charts, componentPaths.Values)

		if isSkeleton {
			if chart.LocalPath != "" {
				rel := filepath.Join(layout.ChartsDir, fmt.Sprintf("%s-%d", chart.Name, chartIdx))
				dst := filepath.Join(componentPaths.Base, rel)

				err := utils.CreatePathAndCopy(chart.LocalPath, dst)
				if err != nil {
					return err
				}

				p.cfg.Pkg.Components[index].Charts[chartIdx].LocalPath = rel
			}

			for valuesIdx, path := range chart.ValuesFiles {
				if helpers.IsURL(path) {
					continue
				}

				rel := fmt.Sprintf("%s-%d", helm.StandardName(layout.ValuesDir, chart), valuesIdx)
				p.cfg.Pkg.Components[index].Charts[chartIdx].ValuesFiles[valuesIdx] = rel

				if err := utils.CreatePathAndCopy(path, filepath.Join(componentPaths.Base, rel)); err != nil {
					return fmt.Errorf("unable to copy chart values file %s: %w", path, err)
				}
			}
		} else {
			err := helmCfg.PackageChart(componentPaths.Charts)
			if err != nil {
				return err
			}
		}
	}

	for filesIdx, file := range component.Files {
		message.Debugf("Loading %#v", file)

		rel := filepath.Join(layout.FilesDir, strconv.Itoa(filesIdx), filepath.Base(file.Target))
		dst := filepath.Join(componentPaths.Base, rel)
		destinationDir := filepath.Dir(dst)

		if helpers.IsURL(file.Source) {
			if isSkeleton {
				continue
			}

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

		if isSkeleton {
			// Change the source to the new relative source directory (any remote files will have been skipped above)
			p.cfg.Pkg.Components[index].Files[filesIdx].Source = rel
			// Remove the extractPath from a skeleton since it will already extract it
			p.cfg.Pkg.Components[index].Files[filesIdx].ExtractPath = ""
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
				if isSkeleton {
					continue
				}
				if err := utils.DownloadToFile(data.Source, dst, component.DeprecatedCosignKeyPath); err != nil {
					return fmt.Errorf(lang.ErrDownloading, data.Source, err.Error())
				}
			} else {
				if err := utils.CreatePathAndCopy(data.Source, dst); err != nil {
					return fmt.Errorf("unable to copy data injection %s: %s", data.Source, err.Error())
				}
				if isSkeleton {
					p.cfg.Pkg.Components[index].DataInjections[dataIdx].Source = rel
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
		for manifestIdx, manifest := range component.Manifests {
			for fileIdx, path := range manifest.Files {
				rel := filepath.Join(layout.ManifestsDir, fmt.Sprintf("%s-%d.yaml", manifest.Name, fileIdx))
				dst := filepath.Join(componentPaths.Base, rel)

				// Copy manifests without any processing.
				spinner.Updatef("Copying manifest %s", path)
				if helpers.IsURL(path) {
					if isSkeleton {
						continue
					}
					if err := utils.DownloadToFile(path, dst, component.DeprecatedCosignKeyPath); err != nil {
						return fmt.Errorf(lang.ErrDownloading, path, err.Error())
					}
				} else {
					if err := utils.CreatePathAndCopy(path, dst); err != nil {
						return fmt.Errorf("unable to copy manifest %s: %w", path, err)
					}
					if isSkeleton {
						p.cfg.Pkg.Components[index].Manifests[manifestIdx].Files[fileIdx] = rel
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
				if isSkeleton {
					p.cfg.Pkg.Components[index].Manifests[manifestIdx].Files = append(p.cfg.Pkg.Components[index].Manifests[manifestIdx].Files, rel)
				}
			}
			if isSkeleton {
				// remove kustomizations
				p.cfg.Pkg.Components[index].Manifests[manifestIdx].Kustomizations = nil
			}
		}
		spinner.Success()
	}

	// Load all specified git repos.
	if len(component.Repos) > 0 && !isSkeleton {
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

	if !isSkeleton {
		if err := p.runActions(onCreate.Defaults, onCreate.After, nil); err != nil {
			return fmt.Errorf("unable to run component after action: %w", err)
		}
	}

	return nil
}

// generateChecksum walks through all of the files starting at the base path and generates a checksum file.
// Each file within the basePath represents a layer within the Zarf package.
// generateChecksum returns a SHA256 checksum of the checksums.txt file.
func (p *Packager) generatePackageChecksums() (string, error) {
	// Loop over the "loaded" files
	var checksumsData = []string{}
	for rel, abs := range p.layout.Files() {
		if rel == layout.ZarfYAML || rel == layout.Checksums {
			continue
		}

		sum, err := utils.GetSHA256OfFile(abs)
		if err != nil {
			return "", err
		}
		checksumsData = append(checksumsData, fmt.Sprintf("%s %s", sum, rel))
	}
	slices.Sort(checksumsData)

	// Create the checksums file
	checksumsFilePath := p.layout.Checksums
	if err := utils.WriteFile(checksumsFilePath, []byte(strings.Join(checksumsData, "\n")+"\n")); err != nil {
		return "", err
	}

	// Calculate the checksum of the checksum file
	return utils.GetSHA256OfFile(checksumsFilePath)
}

// loadDifferentialData extracts the zarf config of a designated 'reference' package that we are building a differential over and creates a list of all images and repos that are in the reference package
func (p *Packager) loadDifferentialData() error {
	// Save the fact that this is a differential build into the build data of the package
	p.cfg.Pkg.Build.Differential = true

	tmpDir, _ := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	defer os.RemoveAll(tmpDir)

	// Load the package spec of the package we're using as a 'reference' for the differential build
	if helpers.IsOCIURL(p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath) {
		remote, err := oci.NewOrasRemote(p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath, oci.PlatformForArch(config.GetArch()))
		if err != nil {
			return err
		}
		pkg, err := remote.FetchZarfYAML()
		if err != nil {
			return err
		}
		err = utils.WriteYaml(filepath.Join(tmpDir, layout.ZarfYAML), pkg, 0600)
		if err != nil {
			return err
		}
	} else {
		if err := archiver.Extract(p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath, layout.ZarfYAML, tmpDir); err != nil {
			return fmt.Errorf("unable to extract the differential zarf package spec: %s", err.Error())
		}
	}

	var differentialZarfConfig types.ZarfPackage
	if err := utils.ReadYaml(filepath.Join(tmpDir, layout.ZarfYAML), &differentialZarfConfig); err != nil {
		return fmt.Errorf("unable to load the differential zarf package spec: %s", err.Error())
	}

	// Generate a map of all the images and repos that are included in the provided package
	allIncludedImagesMap := map[string]bool{}
	allIncludedReposMap := map[string]bool{}
	for _, component := range differentialZarfConfig.Components {
		for _, image := range component.Images {
			allIncludedImagesMap[image] = true
		}
		for _, repo := range component.Repos {
			allIncludedReposMap[repo] = true
		}
	}

	p.cfg.CreateOpts.DifferentialData.DifferentialImages = allIncludedImagesMap
	p.cfg.CreateOpts.DifferentialData.DifferentialRepos = allIncludedReposMap
	p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion = differentialZarfConfig.Metadata.Version

	return nil
}

// removeCopiesFromDifferentialPackage will remove any images and repos that are already included in the reference package from the new package
func (p *Packager) removeCopiesFromDifferentialPackage() error {
	// If a differential build was not requested, continue on as normal
	if p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath == "" {
		return nil
	}

	// Loop through all of the components to determine if any of them are using already included images or repos
	componentMap := make(map[int]types.ZarfComponent)
	for idx, component := range p.cfg.Pkg.Components {
		newImageList := []string{}
		newRepoList := []string{}
		// Generate a list of all unique images for this component
		for _, img := range component.Images {
			// If a image doesn't have a ref (or is a commonly reused ref), we will include this image in the differential package
			imgRef, err := transform.ParseImageRef(img)
			if err != nil {
				return fmt.Errorf("unable to parse image ref %s: %s", img, err.Error())
			}

			// Only include new images or images that have a commonly overwritten tag
			imgTag := imgRef.TagOrDigest
			useImgAnyways := imgTag == ":latest" || imgTag == ":stable" || imgTag == ":nightly"
			if useImgAnyways || !p.cfg.CreateOpts.DifferentialData.DifferentialImages[img] {
				newImageList = append(newImageList, img)
			} else {
				message.Debugf("Image %s is already included in the differential package", img)
			}
		}

		// Generate a list of all unique repos for this component
		for _, repoURL := range component.Repos {
			// Split the remote url and the zarf reference
			_, refPlain, err := transform.GitURLSplitRef(repoURL)
			if err != nil {
				return err
			}

			var ref plumbing.ReferenceName
			// Parse the ref from the git URL.
			if refPlain != "" {
				ref = git.ParseRef(refPlain)
			}

			// Only include new repos or repos that were not referenced by a specific commit sha or tag
			useRepoAnyways := ref == "" || (!ref.IsTag() && !plumbing.IsHash(refPlain))
			if useRepoAnyways || !p.cfg.CreateOpts.DifferentialData.DifferentialRepos[repoURL] {
				newRepoList = append(newRepoList, repoURL)
			} else {
				message.Debugf("Repo %s is already included in the differential package", repoURL)
			}
		}

		// Update the component with the unique lists of repos and images
		component.Images = newImageList
		component.Repos = newRepoList
		componentMap[idx] = component
	}

	// Update the package with the new component list
	for idx, component := range componentMap {
		p.cfg.Pkg.Components[idx] = component
	}

	return nil
}
