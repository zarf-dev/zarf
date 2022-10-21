package generator

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/uuid"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func getOrAskNamespace(source string, componentType string) (namespace string) {
	if config.GenerateOptions.Namespace != "" {
		return config.GenerateOptions.Namespace
	} else {
		prompt := &survey.Input{
			Message: fmt.Sprintf("What namespace would you like to use for your %s component from %s?", componentType, source),
		}

		if err := survey.AskOne(prompt, &namespace, survey.WithValidator(survey.Required)); err != nil {
			message.Fatal("", err.Error())
		}
		return namespace
	}
}

func GenLocalChart(path string, packageName string) (newComponent types.ZarfComponent) {
	chart, err := loader.LoadDir(path)
	if err != nil {
		message.Fatal(err, "Error loading chart")
	}
	namespace := getOrAskNamespace(path, "local chart")
	newComponent.Name = packageName + "-chart-" + strings.ToLower(chart.Name()) + "-" + uuid.NewString()
	newChart := types.ZarfChart{
		Name:    chart.Name(),
		Version: chart.Metadata.Version,
		Namespace: namespace,
		LocalPath: path,
	}
	newComponent.Charts = append(newComponent.Charts, newChart)
	return newComponent
}

func GenManifests(path string) (newComponent types.ZarfComponent) {
	return newComponent
}

func GenLocalFiles(path string) (newComponent types.ZarfComponent) {
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