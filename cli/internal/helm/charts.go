package helm

import (
	"os"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"

	"strings"

	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
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

	var out strings.Builder

	// Setup the helm pull config
	pull := action.NewPull()
	pull.Settings = cli.New()

	// Setup the chart downloader
	downloader := downloader.ChartDownloader{
		Out:              &out,
		Verify:           downloader.VerifyIfPossible,
		Getters:          getter.All(pull.Settings),
	}

	// @todo: process OCI-based charts 

	// Perform simple chart download
	chartURL, err := repo.FindChartInRepoURL(chart.Url, chart.Name, chart.Version, pull.CertFile, pull.KeyFile, pull.CaFile, getter.All(pull.Settings))
	if err != nil {
		logContext.Fatal("Unable to pull the helm chart")
	}

	// Download the file (we don't control what name helm creates here)
	saved, _, err := downloader.DownloadTo(chartURL, pull.Version, destination)
	if err != nil {
		logContext.Fatal("Unable to download the helm chart")
	}

	// Ensure the name is consistent for deployments
	destinationTarball := StandardName(destination, chart)
	os.Rename(saved, destinationTarball)
}

// StandardName generates a predictable full path for a helm chart for Zarf
func StandardName(destintation string, chart config.ZarfChart) string {
	return destintation + "/" + chart.Name + "-" + chart.Version + ".tgz"
}
