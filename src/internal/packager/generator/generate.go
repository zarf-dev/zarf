package generator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/uuid"
	helmChart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
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

func getOrAskNamespace(source string, componentType string, required bool) (namespace string) {
	if config.GenerateOptions.Namespace != "" {
		return config.GenerateOptions.Namespace
	} else {
		prompt := &survey.Input{
			Message: fmt.Sprintf("What namespace would you like to use for your %s component from %s?", componentType, source),
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
		message.Info("TEST" + currentYaml.Kind)
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
	chart, err := loader.LoadDir(path)
	if err != nil {
		message.Fatal(err, "Error loading chart")
	}
	namespace := getOrAskNamespace(path, "local chart", true)
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
	namespace := getOrAskNamespace(path, "manifests", false)
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
		dest := askQuestion("What is the destination for "+file+"?", true)
		newZarfFile := types.ZarfFile{
			Source: file,
			Target: dest,
		}
		newComponent.Files = append(newComponent.Files, newZarfFile)
	}
	return newComponent
}

func GenGitChart(url string) (newComponent types.ZarfComponent) {
	newComponent.Name = "component-git-chart-" + uuid.NewString()
	newComponent.Repos = append(newComponent.Repos, url)
	tempDirPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		message.Fatalf(err, "Unable to create tmpdir:  %s", config.CommonOptions.TempDirectory)
	}

	spinner := message.NewProgressSpinner("Loading git repo")

	repo := git.New(types.GitServerInfo{
		Address: url,
	})

	err = repo.Pull(url, tempDirPath, false)
	if err != nil {
		message.Fatalf(err, fmt.Sprintf("Unable to pull the repo with the url of (%s}", url))
	}
	spinner.Success()

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
	if len(charts) > 1 {
		prompt := &survey.MultiSelect{
			Message: "Please select the charts you want included:",
			Options: transformSlice(charts, func(c chartWithPath) string { return c.chart.Name() }),
		}
		err = survey.AskOne(prompt, &selectedChartNames, survey.WithValidator(survey.Required))
		if err != nil {
			message.Fatalf("Survey error", err.Error())
		}
		for _, selectedChartName := range selectedChartNames {
			filteredChartsWithPaths = append(filteredChartsWithPaths, filterSlice(charts, func(c chartWithPath) bool { return c.chart.Name() == selectedChartName })...)
		}
	} else {
		filteredChartsWithPaths = charts
	}

	for _, chartWithPath := range filteredChartsWithPaths {
		prompt := &survey.Input{
			Message: fmt.Sprintf("What namespace would you like to use for %s?", chartWithPath.chart.Name()),
		}
		var namespace string
		err = survey.AskOne(prompt, &namespace, survey.WithValidator(survey.Required))
		if err != nil {
			message.Fatalf("Survey error", err.Error())
		}
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

func GenHelmRepoChart(path string) (newComponent types.ZarfComponent) {
	return newComponent
}

func GenRemoteFile(path string) (newComponent types.ZarfComponent) {
	return newComponent
}
