// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"crypto"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Create(baseDir string) error {
	var originalDir string

	// Change the working directory if this run has an alternate base dir
	if baseDir != "" {
		originalDir, _ = os.Getwd()
		if err := os.Chdir(baseDir); err != nil {
			return fmt.Errorf("unable to access directory '%s': %w", baseDir, err)
		}
		message.Note(fmt.Sprintf("Using build directory %s", baseDir))
	}

	if err := p.readYaml(config.ZarfYAML, false); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml file: %w", err)
	}

	if p.cfg.Pkg.Kind == "ZarfInitConfig" {
		p.cfg.IsInitConfig = true
	}

	if err := p.composeComponents(); err != nil {
		return err
	}

	// After components are composed, template the active package
	if err := p.fillActiveTemplate(); err != nil {
		return fmt.Errorf("unable to fill variables in template: %s", err.Error())
	}

	seedImage := fmt.Sprintf("%s:%s", config.ZarfSeedImage, config.ZarfSeedTag)

	// Add the seed image to the registry component if this is an init config.
	if p.cfg.IsInitConfig {
		for idx, c := range p.cfg.Pkg.Components {
			if c.Name == "zarf-registry" {
				p.cfg.Pkg.Components[idx].Images = append(c.Images, seedImage)
			}
		}
	}

	// Save the transformed config
	if err := p.writeYaml(); err != nil {
		return fmt.Errorf("unable to write zarf.yaml: %w", err)
	}

	// Perform early package validation
	if err := validate.Run(p.cfg.Pkg); err != nil {
		return fmt.Errorf("unable to validate package: %w", err)
	}

	if !p.confirmAction("Create", nil) {
		return fmt.Errorf("package creation canceled")
	}

	// Save the seed image as an OCI image if this is an init config.
	if p.cfg.IsInitConfig {
		spinner := message.NewProgressSpinner("Loading Zarf Registry Seed Image")
		defer spinner.Stop()

		ociPath := path.Join(p.tmp.Base, "seed-image")
		imgConfig := images.ImgConfig{
			Insecure: config.CommonOptions.Insecure,
		}

		image, err := imgConfig.PullImage(seedImage, spinner)
		if err != nil {
			return fmt.Errorf("unable to pull seed image: %w", err)
		}

		if err := crane.SaveOCI(image, ociPath); err != nil {
			return fmt.Errorf("unable to save image %s as OCI: %w", image, err)
		}

		spinner.Success()
	}

	var combinedImageList []string
	componentSBOMs := map[string]*types.ComponentSBOM{}
	for _, component := range p.cfg.Pkg.Components {
		componentSBOM, err := p.addComponent(component)
		onCreate := component.Actions.OnCreate
		onFailure := func() {
			if err := p.runActions(onCreate.Defaults, onCreate.OnFailure, nil); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}

		if err != nil {
			onFailure()
			return fmt.Errorf("unable to add component: %w", err)
		}

		if err := p.runActions(onCreate.Defaults, onCreate.OnSuccess, nil); err != nil {
			onFailure()
			return fmt.Errorf("unable to run component success action: %w", err)
		}

		if componentSBOM != nil && len(componentSBOM.Files) > 0 {
			componentSBOMs[component.Name] = componentSBOM
		}

		// Combine all component images into a single entry for efficient layer reuse
		combinedImageList = append(combinedImageList, component.Images...)
	}

	imgList := utils.Unique(combinedImageList)

	// Images are handled separately from other component assets
	if len(imgList) > 0 {
		message.HeaderInfof("📦 COMPONENT IMAGES")

		doPull := func() error {
			imgConfig := images.ImgConfig{
				ImagesPath: p.tmp.Images,
				ImgList:    imgList,
				Insecure:   config.CommonOptions.Insecure,
			}

			return imgConfig.PullAll()
		}

		if err := utils.Retry(doPull, 3, 5*time.Second); err != nil {
			return fmt.Errorf("unable to pull images after 3 attempts: %w", err)
		}
	}

	// Ignore SBOM creation if there the flag is set
	if p.cfg.CreateOpts.SkipSBOM {
		message.Debug("Skipping image SBOM processing per --skip-sbom flag")
	} else {
		if err := sbom.Catalog(componentSBOMs, imgList, p.tmp); err != nil {
			return fmt.Errorf("unable to create an SBOM catalog for the package: %w", err)
		}
	}

	// Process the component directories into compressed tarballs
	// NOTE: This is purposefully being done after the SBOM cataloging
	for _, component := range p.cfg.Pkg.Components {
		// Make the component a tar.zst archive
		componentPaths, _ := p.createComponentPaths(component)
		componentName := fmt.Sprintf("%s.%s", component.Name, "tar")
		componentTarPath := filepath.Join(p.tmp.Components, componentName)
		if err := archiver.Archive([]string{componentPaths.Base}, componentTarPath); err != nil {
			return fmt.Errorf("unable to create package: %w", err)
		}

		// Remove the deflated component directory
		if err := os.RemoveAll(componentPaths.Base); err != nil {
			message.Debugf("unable to remove the component directory (%s): %s", componentPaths.Base, err.Error())
		}
	}

	// In case the directory was changed, reset to prevent breaking relative target paths
	if originalDir != "" {
		_ = os.Chdir(originalDir)
	}

	// Use the output path if the user specified it.
	packageName := filepath.Join(p.cfg.CreateOpts.OutputDirectory, p.GetPackageName())

	// Try to remove the package if it already exists.
	_ = os.RemoveAll(packageName)

	// Make the archive
	archiveSrc := []string{p.tmp.Base + string(os.PathSeparator)}
	if err := archiver.Archive(archiveSrc, packageName); err != nil {
		return fmt.Errorf("unable to create package: %w", err)
	}

	f, err := os.Stat(packageName)
	if err != nil {
		return fmt.Errorf("unable to read the package archive: %w", err)
	}

	// Convert Megabytes to bytes
	chunkSize := p.cfg.CreateOpts.MaxPackageSizeMB * 1000 * 1000

	// If a chunk size was specified and the package is larger than the chunk size, split it into chunks
	if p.cfg.CreateOpts.MaxPackageSizeMB > 0 && f.Size() > int64(chunkSize) {
		chunks, sha256sum, err := utils.SplitFile(packageName, chunkSize)
		if err != nil {
			return fmt.Errorf("unable to split the package archive into multiple files: %w", err)
		}
		if len(chunks) > 999 {
			return fmt.Errorf("unable to split the package archive into multiple files: must be less than 1,000 files")
		}

		message.Infof("Package split into %d files, original sha256sum is %s", len(chunks)+1, sha256sum)
		_ = os.RemoveAll(packageName)

		// Marshal the data into a json file
		jsonData, err := json.Marshal(types.ZarfPartialPackageData{
			Count:     len(chunks),
			Bytes:     f.Size(),
			Sha256Sum: sha256sum,
		})
		if err != nil {
			return fmt.Errorf("unable to marshal the partial package data: %w", err)
		}

		// Prepend the json data to the first chunk
		chunks = append([][]byte{jsonData}, chunks...)

		for idx, chunk := range chunks {
			path := fmt.Sprintf("%s.part%03d", packageName, idx)
			if err := os.WriteFile(path, chunk, 0644); err != nil {
				return fmt.Errorf("unable to write the file %s: %w", path, err)
			}
		}
	}

	// Output the SBOM files into a directory if specified
	if p.cfg.CreateOpts.SBOMOutputDir != "" {
		if err := sbom.OutputSBOMFiles(p.tmp, p.cfg.CreateOpts.SBOMOutputDir, p.cfg.Pkg.Metadata.Name); err != nil {
			return err
		}
	}

	// Open a browser to view the SBOM if specified
	if p.cfg.CreateOpts.ViewSBOM {
		sbom.ViewSBOMFiles(p.tmp)
	}

	return nil
}

func (p *Packager) addComponent(component types.ZarfComponent) (*types.ComponentSBOM, error) {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	// Create the component directory.
	componentPath, err := p.createComponentPaths(component)
	if err != nil {
		return nil, fmt.Errorf("unable to create component paths: %w", err)
	}

	// Create an struct to hold the SBOM information for this component
	componentSBOM := types.ComponentSBOM{
		Files:         []string{},
		ComponentPath: componentPath,
	}

	onCreate := component.Actions.OnCreate

	if err := p.runActions(onCreate.Defaults, onCreate.Before, nil); err != nil {
		return nil, fmt.Errorf("unable to run component before action: %w", err)
	}

	// If any helm charts are defined, process them.
	if len(component.Charts) > 0 {
		_ = utils.CreateDirectory(componentPath.Charts, 0700)
		_ = utils.CreateDirectory(componentPath.Values, 0700)
		re := regexp.MustCompile(`\.git$`)

		for _, chart := range component.Charts {
			isGitURL := re.MatchString(chart.URL)
			helmCfg := helm.Helm{
				Chart: chart,
				Cfg:   p.cfg,
			}

			if isGitURL {
				_ = helmCfg.DownloadChartFromGit(componentPath.Charts)
			} else if len(chart.URL) > 0 {
				helmCfg.DownloadPublishedChart(componentPath.Charts)
			} else {
				path := helmCfg.CreateChartFromLocalFiles(componentPath.Charts)
				zarfFilename := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
				if !strings.HasSuffix(path, zarfFilename) {
					return nil, fmt.Errorf("error creating chart archive, user provided chart name and/or version does not match given chart")
				}
			}

			for idx, path := range chart.ValuesFiles {
				chartValueName := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(idx)
				if err := utils.CreatePathAndCopy(path, chartValueName); err != nil {
					return nil, fmt.Errorf("unable to copy chart values file %s: %w", path, err)
				}
			}
		}
	}

	if len(component.Files) > 0 {
		_ = utils.CreateDirectory(componentPath.Files, 0700)

		for index, file := range component.Files {
			message.Debugf("Loading %#v", file)
			destinationFile := filepath.Join(componentPath.Files, strconv.Itoa(index))

			if utils.IsURL(file.Source) {
				utils.DownloadToFile(file.Source, destinationFile, component.CosignKeyPath)
			} else {
				if err := utils.CreatePathAndCopy(file.Source, destinationFile); err != nil {
					return nil, fmt.Errorf("unable to copy file %s: %w", file.Source, err)
				}
			}

			// Abort packaging on invalid shasum (if one is specified)
			if file.Shasum != "" {
				if actualShasum, _ := utils.GetCryptoHash(destinationFile, crypto.SHA256); actualShasum != file.Shasum {
					return nil, fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", file.Source, file.Shasum, actualShasum)
				}
			}

			info, _ := os.Stat(destinationFile)

			if file.Executable || info.IsDir() {
				_ = os.Chmod(destinationFile, 0700)
			} else {
				_ = os.Chmod(destinationFile, 0600)
			}

			componentSBOM.Files = append(componentSBOM.Files, destinationFile)
		}
	}

	if len(component.DataInjections) > 0 {
		spinner := message.NewProgressSpinner("Loading data injections")
		defer spinner.Success()

		for _, data := range component.DataInjections {
			spinner.Updatef("Copying data injection %s for %s", data.Target.Path, data.Target.Selector)
			destination := filepath.Join(componentPath.DataInjections, filepath.Base(data.Target.Path))
			if err := utils.CreatePathAndCopy(data.Source, destination); err != nil {
				return nil, fmt.Errorf("unable to copy data injection %s: %w", data.Source, err)
			}

			// Unwrap the dataInjection dir into individual files
			pattern := regexp.MustCompile(`(?mi).+$`)
			files, _ := utils.RecursiveFileList(destination, pattern)
			componentSBOM.Files = append(componentSBOM.Files, files...)
		}
	}

	if len(component.Manifests) > 0 {
		// Get the proper count of total manifests to add
		manifestCount := 0

		for _, manifest := range component.Manifests {
			manifestCount += len(manifest.Files)
			manifestCount += len(manifest.Kustomizations)
		}

		spinner := message.NewProgressSpinner("Loading %d K8s manifests", manifestCount)
		defer spinner.Success()

		if err := utils.CreateDirectory(componentPath.Manifests, 0700); err != nil {
			return nil, fmt.Errorf("unable to create manifest directory %s: %w", componentPath.Manifests, err)
		}

		// Iterate over all manifests
		for _, manifest := range component.Manifests {
			for _, f := range manifest.Files {
				// Copy manifests without any processing
				spinner.Updatef("Copying manifest %s", f)
				destination := fmt.Sprintf("%s/%s", componentPath.Manifests, f)
				if err := utils.CreatePathAndCopy(f, destination); err != nil {
					return nil, fmt.Errorf("unable to copy manifest %s: %w", f, err)
				}
			}

			for idx, k := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package
				spinner.Updatef("Building kustomization for %s", k)
				destination := fmt.Sprintf("%s/kustomization-%s-%d.yaml", componentPath.Manifests, manifest.Name, idx)
				if err := kustomize.BuildKustomization(k, destination, manifest.KustomizeAllowAnyDirectory); err != nil {
					return nil, fmt.Errorf("unable to build kustomization %s: %w", k, err)
				}
			}
		}
	}

	// Load all specified git repos
	if len(component.Repos) > 0 {
		spinner := message.NewProgressSpinner("Loading %d git repos", len(component.Repos))
		defer spinner.Success()

		for _, url := range component.Repos {
			// Pull all the references if there is no `@` in the string
			gitCfg := git.NewWithSpinner(p.cfg.State.GitServer, spinner)
			if err := gitCfg.Pull(url, componentPath.Repos); err != nil {
				return nil, fmt.Errorf("unable to pull git repo %s: %w", url, err)
			}
		}
	}

	if err := p.runActions(onCreate.Defaults, onCreate.After, nil); err != nil {
		return nil, fmt.Errorf("unable to run component after action: %w", err)
	}

	return &componentSBOM, nil
}
