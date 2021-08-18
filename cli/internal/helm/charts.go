package helm

import (
	"os"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
)

func DownloadChartFromGit(chart config.ZarfChart, destination string) {
	logContext := logrus.WithFields(logrus.Fields{
		"Chart":   chart.Name,
		"URL":     chart.Url,
		"Version": chart.Version,
	})

	logContext.Info("Processing git-based helm chart")
	client := action.NewPackage()

	// Get the git repo
	tempPath := git.DownloadRepoToTemp(chart.Url)

	// Switch to the correct tag
	git.CheckoutTag(tempPath, chart.Version)

	// Tell helm where to save the archive and create the package
	client.Destination = destination
	client.Run(tempPath+"/chart", nil)

	_ = os.RemoveAll(tempPath)
}

func DownloadPublishedChart(chart config.ZarfChart, destination string) {
	logContext := logrus.WithFields(logrus.Fields{
		"Chart":   chart.Name,
		"URL":     chart.Url,
		"Version": chart.Version,
	})

	logContext.Info("Processing published helm chart")
	client := action.NewPull()
	client.Settings = cli.New()
	client.DestDir = destination
	client.Version = chart.Version
	client.RepoURL = chart.Url
	_, err := client.Run(chart.Name)
	if err != nil {
		logContext.Fatal("Unable to load the helm chart")
	}
}
