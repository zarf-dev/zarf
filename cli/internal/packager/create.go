package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/cli/internal/kustomize"
	"github.com/defenseunicorns/zarf/cli/internal/packager/validate"
	"github.com/defenseunicorns/zarf/cli/types"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/helm"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
)

// Create generates a zarf package tarball for consumption by
func Create() {
	if err := config.LoadConfig("zarf.yaml"); err != nil {
		message.Fatal(err, "Unable to read the zarf.yaml file")
	}

	tempPath := createPaths()
	defer tempPath.clean()

	components := GetComposedComponents()
	seedImages := config.GetSeedImages()
	packageName := config.GetPackageName()
	dataInjections := config.GetDataInjections()
	configFile := tempPath.base + "/zarf.yaml"

	config.SetAcrch()

	// Save the transformed config
	if err := config.BuildConfig(configFile); err != nil {
		message.Fatalf(err, "Unable to write the %s file", configFile)
	}

	// Perform early package validation
	validate.Run()

	if !confirmAction(configFile, "Create") {
		os.Exit(0)
	}

	if len(seedImages) > 0 {
		// Load seed images into their own happy little tarball for ease of import on init
		images.PullAll(seedImages, tempPath.seedImages)
	}

	var combinedImageList []string
	for _, component := range components {
		addComponent(tempPath, component)
		// Combine all component images into a single entry for efficient layer reuse
		combinedImageList = append(combinedImageList, component.Images...)
	}

	// Images are handled separately from other component assets
	if len(combinedImageList) > 0 {
		uniqueList := removeDuplicates(combinedImageList)
		images.PullAll(uniqueList, tempPath.images)
	}

	if config.IsZarfInitConfig() {
		// Override the package name for init packages
		packageName = config.PackageInitName
	} else {
		// Init packages do not use data or utilityCluster keys
		if len(dataInjections) > 0 {
			for _, data := range dataInjections {
				destinationFile := tempPath.dataInjections + "/" + filepath.Base(data.Target.Path)
				utils.CreatePathAndCopy(data.Source, destinationFile)
			}
		}
	}
	_ = os.RemoveAll(packageName)
	err := archiver.Archive([]string{tempPath.base + "/"}, packageName)
	if err != nil {
		message.Fatal(err, "Unable to create the package archive")
	}
}

func addComponent(tempPath tempPaths, component types.ZarfComponent) {
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))
	componentPath := createComponentPaths(tempPath.components, component)

	if len(component.Charts) > 0 {
		_ = utils.CreateDirectory(componentPath.charts, 0700)
		_ = utils.CreateDirectory(componentPath.values, 0700)
		re := regexp.MustCompile(`\.git$`)
		for _, chart := range component.Charts {
			isGitURL := re.MatchString(chart.Url)
			if isGitURL {
				_ = helm.DownloadChartFromGit(chart, componentPath.charts)
			} else {
				helm.DownloadPublishedChart(chart, componentPath.charts)
			}
			for idx, path := range chart.ValuesFiles {
				chartValueName := helm.StandardName(componentPath.values, chart) + "-" + strconv.Itoa(idx)
				utils.CreatePathAndCopy(path, chartValueName)
			}
		}
	}

	if len(component.Files) > 0 {
		_ = utils.CreateDirectory(componentPath.files, 0700)
		for index, file := range component.Files {
			message.Debugf("Loading %v", file)
			destinationFile := componentPath.files + "/" + strconv.Itoa(index)
			if utils.IsUrl(file.Source) {
				utils.DownloadToFile(file.Source, destinationFile)
			} else {
				utils.CreatePathAndCopy(file.Source, destinationFile)
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

	if len(component.Manifests) > 0 {
		spinner := message.NewProgressSpinner("Loading %d manifests", len(component.Manifests))
		defer spinner.Stop()

		if err := utils.CreateDirectory(componentPath.manifests, 0700); err != nil {
			spinner.Fatalf(err, "Unable to create the manifest path %s", componentPath.manifests)
		}

		// Iterate over all manifests
		for _, manifest := range component.Manifests {
			for _, file := range manifest.Files {
				// Copy manifests without any processing
				spinner.Updatef("Copying manifest %s", file)
				destination := fmt.Sprintf("%s/%s", componentPath.manifests, file)
				utils.CreatePathAndCopy(file, destination)
			}
			for idx, kustomization := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package
				spinner.Updatef("Building kustomization for %s", kustomization)
				destination := fmt.Sprintf("%s/kustomization-%s-%d.yaml", componentPath.manifests, manifest.Name, idx)
				if err := kustomize.BuildKustomization(kustomization, destination); err != nil {
					spinner.Fatalf(err, "unable to build the kustomization for %s", kustomization)
				}
			}
		}
		spinner.Success()
	}

	// Load all specified git repos
	if len(component.Repos) > 0 {
		spinner := message.NewProgressSpinner("Loading %v git repos", len(component.Repos))
		defer spinner.Stop()
		for _, url := range component.Repos {
			// Pull all the references if there is no `@` in the string
			git.Pull(url, componentPath.repos, spinner)
		}
		spinner.Success()
	}
}
