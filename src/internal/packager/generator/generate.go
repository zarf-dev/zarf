package generator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/uuid"
	"helm.sh/helm/v3/pkg/action"
	helmChart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

type chartWithPath struct {
	chart helmChart.Chart
	path  string
}

type yamlKind struct {
	Kind string `json:"kind"`
}

func checkStringContainsInSlice(stringSlice []string, match string) bool {
	for _, s := range stringSlice {
		dir, _ := filepath.Split(s)
		if strings.Contains(match, dir) {
			return true
		}
	}
	return false
}

func filterSlice[Type any](slice []Type, filterFunction func(Type) bool) []Type {
	var newSlice []Type
	for _, element := range slice {
		if filterFunction(element) {
			newSlice = append(newSlice, element)
		}
	}
	return newSlice
}

func getOrAskNamespace(source string, componentType string, required bool, defaultNS string) (namespace string) {
	if config.CommonOptions.Confirm {
		return defaultNS
	} else if config.GenerateOptions.Namespace != "" {
		return config.GenerateOptions.Namespace
	} else {
		prompt := &survey.Input{
			Message: fmt.Sprintf("What namespace would you like to use for your %s component from %s?", componentType, source),
			Default: defaultNS,
		}
		if required {
			if err := survey.AskOne(prompt, &namespace, survey.WithValidator(survey.Required)); err != nil {
				message.Fatal("", err.Error())
			}
		} else {
			prompt.Message = fmt.Sprintf("If you would like a namespace for your %s component from %s, please enter it now:", componentType, source)
			prompt.Help = "You may leave the input blank, the namespace will be inherited from the metadata of the manifests in that case"
			if err := survey.AskOne(prompt, &namespace); err != nil {
				message.Fatal("", err.Error())
			}
		}
		return namespace
	}
}

func separateManifestsAndKustomizations(dirPath string) (manifests []string, kustomizations []string) {
	topLevelFilesPaths := getTopLevelFiles(dirPath)
	yamlFilesPaths := []string{}
	isYaml := regexp.MustCompile(`.*\.yaml$`).MatchString
	for _, file := range topLevelFilesPaths {
		if isYaml(file) {
			yamlFilesPaths = append(yamlFilesPaths, file)
		}
	}
	for _, yamlFile := range yamlFilesPaths {
		var currentYaml yamlKind
		err := utils.ReadYaml(yamlFile, &currentYaml)
		if err != nil {
			message.Fatalf(err, "Error reading manifest %s", yamlFile)
		}
		if currentYaml.Kind != "" {
			if currentYaml.Kind == "Kustomization" {
				kustomizations = append(kustomizations, yamlFile)
			} else if currentYaml.Kind == "ZarfPackageConfig" {
				continue
			} else {
				manifests = append(manifests, yamlFile)
			}
		}
	}
	return manifests, kustomizations
}

func transformSlice[InputType any, ReturnType any](slice []InputType, transformFunction func(InputType) ReturnType) []ReturnType {
	var newSlice []ReturnType
	for _, element := range slice {
		newSlice = append(newSlice, transformFunction(element))
	}
	return newSlice
}

func GenLocalChart(path string) (newComponent types.ZarfComponent) {
	defer message.Successf("Local chart component successfully generated")
	chart, err := loader.LoadDir(path)
	if err != nil {
		message.Fatal(err, "Error loading chart")
	}
	namespace := getOrAskNamespace(path, "local-chart", true, "zarf-generated-local-chart-"+chart.Name())
	newComponent.Name = "component-local-chart-" + strings.ToLower(chart.Name()) + "-" + uuid.NewString()
	newChart := types.ZarfChart{
		Name:      chart.Name(),
		Version:   chart.Metadata.Version,
		Namespace: namespace,
		LocalPath: path,
	}
	newComponent.Charts = append(newComponent.Charts, newChart)
	return newComponent
}

func GenManifests(path string) (newComponent types.ZarfComponent) {
	defer message.Successf("Manifests component successfully generated")
	namespace := getOrAskNamespace(path, "manifests", false, "")
	newComponent.Name = "component-manifests-" + uuid.NewString()
	if isDir(path) {
		manifests, kustomizations := separateManifestsAndKustomizations(path)
		newZarfManifest := types.ZarfManifest{
			Name:           "manifests-" + uuid.NewString(),
			Namespace:      namespace,
			Files:          manifests,
			Kustomizations: kustomizations,
		}
		newComponent.Manifests = append(newComponent.Manifests, newZarfManifest)
	} else {
		newZarfManifest := types.ZarfManifest{
			Name:      "manifests-" + uuid.NewString(),
			Namespace: namespace,
			Files:     []string{path},
		}
		newComponent.Manifests = append(newComponent.Manifests, newZarfManifest)
	}
	return newComponent
}

func GenLocalFiles(path string) (newComponent types.ZarfComponent) {
	defer message.Successf("Local files component successfully generated")
	newComponent.Name = "component-files-" + uuid.NewString()
	var filePaths []string
	if isDir(path) {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			message.Fatal(err, "Error reading directory")
		}
		for _, entry := range dirEntries {
			filePaths = append(filePaths, filepath.Join(path, entry.Name()))
		}
	} else {
		filePaths = append(filePaths, path)
	}

	for _, file := range filePaths {
		dest := ""
		if config.CommonOptions.Confirm {
			dest = "/tmp/zarf"
		} else {
			dest = askQuestion("What is the destination for "+file+"?", true, "/tmp/zarf/")
		}
		newZarfFile := types.ZarfFile{
			Source: file,
			Target: dest,
		}
		newComponent.Files = append(newComponent.Files, newZarfFile)
	}
	return newComponent
}

func GenGitChart(url string) (newComponent types.ZarfComponent) {
	defer message.Successf("Git chart component successfully generated")
	newComponent.Name = "component-git-chart-" + uuid.NewString()
	newComponent.Repos = append(newComponent.Repos, url)
	tempDirPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		message.Fatalf(err, "Unable to create tmpdir:  %s", config.CommonOptions.TempDirectory)
	}

	spinner := message.NewProgressSpinner("Loading git repository")

	repo := git.New(types.GitServerInfo{
		Address: url,
	})

	err = repo.Pull(url, tempDirPath, false)
	if err != nil {
		message.Fatalf(err, fmt.Sprintf("Unable to pull the repo with the url of (%s}", url))
	}
	spinner.Successf("Git repository loaded")

	var chartYamlPaths []string

	filepath.WalkDir(tempDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "Chart.yaml" {
			dir, _ := filepath.Split(path)
			if !checkStringContainsInSlice(chartYamlPaths, dir) {
				chartYamlPaths = append(chartYamlPaths, path)
			}
		}
		return nil
	})

	var charts []chartWithPath

	for _, chartYamlPath := range chartYamlPaths {
		dir, _ := filepath.Split(chartYamlPath)
		chart, err := loader.LoadDir(dir)
		if err != nil {
			message.Fatal(err, "Error loading chart")
		}
		newChart := chartWithPath{chart: *chart, path: chartYamlPath}
		charts = append(charts, newChart)
	}

	var selectedChartNames []string
	var filteredChartsWithPaths []chartWithPath
	foundChartNames := transformSlice(charts, func(c chartWithPath) string { return c.chart.Name() })
	if len(charts) > 1 {
		if !config.CommonOptions.Confirm {
			prompt := &survey.MultiSelect{
				Message: "Please select the charts you want included:",
				Options: foundChartNames,
				Default: foundChartNames,
			}
			err = survey.AskOne(prompt, &selectedChartNames, survey.WithValidator(survey.Required))
			if err != nil {
				message.Fatalf("Survey error", err.Error())
			}
		}
		for _, selectedChartName := range selectedChartNames {
			filteredChartsWithPaths = append(filteredChartsWithPaths, filterSlice(charts, func(c chartWithPath) bool { return c.chart.Name() == selectedChartName })...)
		}
	} else {
		filteredChartsWithPaths = charts
	}

	for _, chartWithPath := range filteredChartsWithPaths {
		namespace := getOrAskNamespace("the "+chartWithPath.chart.Name()+" chart", "git repo", true, "zarf-generated-git-chart-"+chartWithPath.chart.Name())
		chartDir, _ := filepath.Split(chartWithPath.path)
		newZarfChart := types.ZarfChart{
			Name:      chartWithPath.chart.Name(),
			Version:   chartWithPath.chart.Metadata.Version,
			Namespace: namespace,
			GitPath:   chartDir,
		}
		newComponent.Charts = append(newComponent.Charts, newZarfChart)
	}

	return newComponent
}

func GenHelmRepoChart(url string) (newComponent types.ZarfComponent) {
	defer message.Successf("Helm repo chart component successfully generated")
	spinner := message.NewProgressSpinner("Loading Helm Repo Entries")
	newComponent.Name = "component-helm-repo-chart-" + uuid.NewString()
	entry := repo.Entry{
		URL: url,
	}

	// Set up the helm pull config
	pull := action.NewPull()
	pull.Settings = cli.New()

	helmRepo, err := repo.NewChartRepository(&entry, getter.All(pull.Settings))
	if err != nil {
		message.Fatalf(err, err.Error())
	}

	cachedIndexPath, err := helmRepo.DownloadIndexFile()
	if err != nil {
		message.Fatalf(err, err.Error())
	}

	helmIndex, err := repo.LoadIndexFile(cachedIndexPath)
	if err != nil {
		message.Fatalf(err, err.Error())
	}

	var chartsInIndex []string
	for k := range helmIndex.Entries {
		chartsInIndex = append(chartsInIndex, k)
	}
	sort.Strings(chartsInIndex)

	spinner.Successf("Loaded Helm Repo Entries")

	var selectedChartNames []string
	if len(helmIndex.Entries) > 1 {
		if config.CommonOptions.Confirm {
			selectedChartNames = chartsInIndex
		} else {
			prompt := &survey.MultiSelect{
				Message: "Please select which chart(s) you would like from the repo:",
				Options: chartsInIndex,
				Default: chartsInIndex,
			}
			err = survey.AskOne(prompt, &selectedChartNames, survey.WithValidator(survey.Required))
			if err != nil {
				message.Fatalf("Survey error", err.Error())
			}
		}
	}

	for _, chartName := range selectedChartNames {
		newZarfChart := types.ZarfChart{
			Name: chartName,
			URL:  url,
		}
		for repoChartName, chartVersion := range helmIndex.Entries {
			if repoChartName == chartName {
				versionList := []string{"latest"}
				for _, version := range chartVersion {
					versionList = append(versionList, version.Version)
				}
				selectedVersion := ""
				if config.CommonOptions.Confirm {
					selectedVersion = "latest"
				} else {
					prompt := &survey.Select{
						Message: "Please select which chart version you would like for " + chartName + ":",
						Options: versionList,
						Default: "latest",
					}
					err = survey.AskOne(prompt, &selectedVersion, survey.WithValidator(survey.Required))
					if err != nil {
						message.Fatalf("Survey error", err.Error())
					}
				}
				if selectedVersion == "latest" {
					selectedVersion = versionList[1]
				}
				newZarfChart.Version = selectedVersion
				newZarfChart.Namespace = getOrAskNamespace(url, chartName+"-chart", true, "zarf-generated-helm-repo-chart-"+chartName)
				break
			}
		}
		newComponent.Charts = append(newComponent.Charts, newZarfChart)
	}

	return newComponent
}

func GenRemoteFile(url string) (newComponent types.ZarfComponent) {
	defer message.Successf("Remote file component successfully generated")
	newComponent.Name = "component-remote-file" + uuid.NewString()
	remoteFileName := strings.Split(strings.Trim(url, "/"), "/")

	remoteFileDest := ""
	if config.CommonOptions.Confirm {
		remoteFileDest = "/tmp/zarf"
	} else {
		prompt := &survey.Input{
			Message: fmt.Sprintf("Where would you like to place %s", remoteFileName[len(remoteFileName)-1]),
			Default: "/tmp/zarf/",
		}
		if err := survey.AskOne(prompt, &remoteFileDest, survey.WithValidator(survey.Required)); err != nil {
			message.Fatal("", err.Error())
		}
	}
	newZarfFile := types.ZarfFile{
		Source: url,
		Target: remoteFileDest,
	}
	newComponent.Files = append(newComponent.Files, newZarfFile)

	return newComponent
}
