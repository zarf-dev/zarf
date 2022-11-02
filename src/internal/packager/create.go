package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Create generates a zarf package tarball for a given PackageConfg and optional base directory.
func (p *Packager) Create(baseDir string) error {
	var originalDir string

	// Change the working directory if this run has an alternate base dir
	if baseDir != "" {
		originalDir, _ = os.Getwd()
		_ = os.Chdir(baseDir)
		message.Note(fmt.Sprintf("Using build directory %s", baseDir))
	}

	if err := p.readYaml(config.ZarfYAML, false); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml file: %w", err)
	}

	if p.cfg.Pkg.Kind == "ZarfInitConfig" {
		p.cfg.IsInitConfig = true
	}

	p.composeComponents()

	// After components are composed, template the active package
	if err := p.fillActiveTemplate(); err != nil {
		return fmt.Errorf("unable to fill variables in template: %s", err.Error())
	}

	// Save the transformed config
	if err := p.writeYaml(); err != nil {
		return fmt.Errorf("unable to write zarf.yaml: %w", err)
	}

	// Perform early package validation
	validate.Run(p.cfg.Pkg)

	if !p.confirmAction("Create", nil) {
		return fmt.Errorf("package creation canceled")
	}

	if p.cfg.IsInitConfig {
		// Load seed images into their own happy little tarball for ease of import on init
		if err := p.pullImages([]string{config.ZarfSeedImage}, p.tmp.SeedImage); err != nil {
			return fmt.Errorf("unable to pull the seed image after 3 attempts: %w", err)
		}
	}

	var combinedImageList []string
	for _, component := range p.cfg.Pkg.Components {
		if err := p.addComponent(component); err != nil {
			return fmt.Errorf("unable to add component: %w", err)
		}

		// Combine all component images into a single entry for efficient layer reuse
		combinedImageList = append(combinedImageList, component.Images...)
	}

	// Images are handled separately from other component assets
	if len(combinedImageList) > 0 {
		uniqueList := utils.Unique(combinedImageList)
		if err := p.pullImages(uniqueList, p.tmp.Images); err != nil {
			return fmt.Errorf("unable to pull images after 3 attempts: %w", err)
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

	return nil
}

func (p *Packager) pullImages(imgList []string, path string) error {
	return utils.Retry(func() error {
		imgConfig := images.ImgConfig{
			TarballPath: path,
			ImgList:     imgList,
			Insecure:    p.cfg.CreateOpts.Insecure,
		}

		pulledImages, err := imgConfig.PullAll()

		if err != nil {
			// Ignore SBOM creation if there the flag is set
			if p.cfg.CreateOpts.SkipSBOM {
				message.Debug("Skipping SBOM processing per --skip-sbom flag")
			} else {
				sbom.CatalogImages(pulledImages, p.tmp.Sboms, path)
			}
		}

		return err
	}, 3, 5*time.Second)
}

func (p *Packager) addComponent(component types.ZarfComponent) error {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	// Create the component directory.
	componentPath, err := p.createComponentPaths(component)
	if err != nil {
		return fmt.Errorf("unable to create component paths: %w", err)
	}

	// Loop through each component prepare script and execute it.
	for _, script := range component.Scripts.Prepare {
		p.loopScriptUntilSuccess(script, component.Scripts)
	}

	// If any helm charts are defined, process them.
	if len(component.Charts) > 0 {
		_ = utils.CreateDirectory(componentPath.Charts, 0700)
		_ = utils.CreateDirectory(componentPath.Values, 0700)
		re := regexp.MustCompile(`\.git$`)

		for _, chart := range component.Charts {
			isGitURL := re.MatchString(chart.Url)
			helmCfg := helm.Helm{
				Chart: chart,
				Cfg:   p.cfg,
			}

			if isGitURL {
				_ = helmCfg.DownloadChartFromGit(componentPath.Charts)
			} else if len(chart.Url) > 0 {
				helmCfg.DownloadPublishedChart(componentPath.Charts)
			} else {
				path := helmCfg.CreateChartFromLocalFiles(componentPath.Charts)
				zarfFilename := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
				if !strings.HasSuffix(path, zarfFilename) {
					return fmt.Errorf("error creating chart archive, user provided chart name and/or version does not match given chart")
				}
			}

			for idx, path := range chart.ValuesFiles {
				chartValueName := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(idx)
				if err := utils.CreatePathAndCopy(path, chartValueName); err != nil {
					return fmt.Errorf("unable to copy chart values file %s: %w", path, err)
				}
			}
		}
	}

	if len(component.Files) > 0 {
		_ = utils.CreateDirectory(componentPath.Files, 0700)

		for index, file := range component.Files {
			message.Debugf("Loading %#v", file)
			destinationFile := filepath.Join(componentPath.Files, strconv.Itoa(index))

			if utils.IsUrl(file.Source) {
				utils.DownloadToFile(file.Source, destinationFile, component.CosignKeyPath)
			} else {
				if err := utils.CreatePathAndCopy(file.Source, destinationFile); err != nil {
					return fmt.Errorf("unable to copy file %s: %w", file.Source, err)
				}
			}

			// Abort packaging on invalid shasum (if one is specified)
			if file.Shasum != "" {
				utils.ValidateSha256Sum(file.Shasum, destinationFile)
			}

			info, _ := os.Stat(destinationFile)

			if file.Executable || info.IsDir() {
				_ = os.Chmod(destinationFile, 0700)
			} else {
				_ = os.Chmod(destinationFile, 0600)
			}
		}
	}

	if len(component.DataInjections) > 0 {
		spinner := message.NewProgressSpinner("Loading data injections")
		defer spinner.Success()

		for _, data := range component.DataInjections {
			spinner.Updatef("Copying data injection %s for %s", data.Target.Path, data.Target.Selector)
			destinationFile := filepath.Join(componentPath.DataInjections, filepath.Base(data.Target.Path))
			if err := utils.CreatePathAndCopy(data.Source, destinationFile); err != nil {
				return fmt.Errorf("unable to copy data injection %s: %w", data.Source, err)
			}
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
			return fmt.Errorf("unable to create manifest directory %s: %w", componentPath.Manifests, err)
		}

		// Iterate over all manifests
		for _, manifest := range component.Manifests {
			for _, f := range manifest.Files {
				// Copy manifests without any processing
				spinner.Updatef("Copying manifest %s", f)
				destination := fmt.Sprintf("%s/%s", componentPath.Manifests, f)
				if err := utils.CreatePathAndCopy(f, destination); err != nil {
					return fmt.Errorf("unable to copy manifest %s: %w", f, err)
				}
			}

			for idx, k := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package
				spinner.Updatef("Building kustomization for %s", k)
				destination := fmt.Sprintf("%s/kustomization-%s-%d.yaml", componentPath.Manifests, manifest.Name, idx)
				if err := kustomize.BuildKustomization(k, destination, manifest.KustomizeAllowAnyDirectory); err != nil {
					return fmt.Errorf("unable to build kustomization %s: %w", k, err)
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
			if _, err := gitCfg.Pull(url, componentPath.Repos); err != nil {
				return fmt.Errorf("unable to pull git repo %s: %w", url, err)
			}
		}
	}

	return nil
}
