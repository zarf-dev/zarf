package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/uuid"
	"helm.sh/helm/v3/pkg/chart/loader"
)

type yamlKind struct {
	Kind string `json:"kind"`
}

func askQuestion(question string, required bool) (answer string) {
	prompt := &survey.Input{
		Message: fmt.Sprint(question),
	}
	var err error
	if required {
		err = survey.AskOne(prompt, &answer, survey.WithValidator(survey.Required))
	} else {
		err = survey.AskOne(prompt, &answer)
	}
	if err != nil {
		message.Fatal("", err.Error())
	}
	return answer
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
		dest := askQuestion("What is the destination for " + file + "?", true)
		newZarfFile := types.ZarfFile{
			Source: file,
			Target: dest,
		}
		newComponent.Files = append(newComponent.Files, newZarfFile)
	}
	return newComponent
}

func GenGitChart(path string) (newComponent types.ZarfComponent) {
	return newComponent
}

func GenHelmRepoChart(path string) (newComponent types.ZarfComponent) {
	return newComponent
}

func GenRemoteFile(path string) (newComponent types.ZarfComponent) {
	return newComponent
}
