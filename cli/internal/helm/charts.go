package helm

import (
	"net/url"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func DownloadChartFromGit(chart config.ZarfChart, destination string) {
	client := action.NewPackage()
	tempPath := git.DownloadRepoToTemp(chart.Url, chart.Version)

	client.Destination = destination
	client.Run(tempPath+"/chart", nil)

	_ = os.RemoveAll(tempPath)
}

func DownloadPublishedChart(chart config.ZarfChart, destination string) {
	chartTarballName := destination + "/" + chart.Name + "-" + chart.Version + ".tgz"
	chartYaml := string(utils.Download(chart.Url + "/index.yaml"))
	yamlPath, _ := yaml.PathString("$.entries." + chart.Name + "[*]")

	var chartTarballUrl string
	var chartData []struct {
		Name    string   `yaml:"name"`
		Urls    []string `yaml:"urls"`
		Version string   `yaml:"version"`
	}

	if err := yamlPath.Read(strings.NewReader(chartYaml), &chartData); err != nil {
		logrus.WithField("chart", chart.Name).Fatal("Unable to process the chart data")
	}

	for _, match := range chartData {
		if match.Version == chart.Version {
			parsedUrl, err := url.Parse(match.Urls[0])
			if err != nil {
				logrus.Warn("Invalid chart URL detected")
			}
			if !parsedUrl.IsAbs() {
				patchUrl, _ := url.Parse(chart.Url)
				parsedUrl.Host = patchUrl.Host
				parsedUrl.Scheme = patchUrl.Scheme
			}
			chartTarballUrl = parsedUrl.String()
			break
		}
	}
	utils.DownloadToFile(chartTarballUrl, chartTarballName)
}
