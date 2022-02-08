package helm

import (
	"github.com/defenseunicorns/zarf/cli/types"
	"os"

	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// DownloadChartFromGit is a special implementation of chart downloads that support the https://p1.dso.mil/#/products/big-bang/ model
func DownloadChartFromGit(chart types.ZarfChart, destination string) {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from git url %s", chart.Name, chart.Version, chart.Url)
	defer spinner.Stop()

	client := action.NewPackage()

	// Get the git repo
	tempPath := git.DownloadRepoToTemp(chart.Url)

	// Switch to the correct tag
	git.CheckoutTag(tempPath, chart.Version)

	// Tell helm where to save the archive and create the package
	client.Destination = destination
	name, err := client.Run(tempPath+"/"+chart.GitPath, nil)

	if err != nil {
		spinner.Fatalf(err, "Helm is unable to save the archive and create the package %s", name)
	}

	_ = os.RemoveAll(tempPath)
	spinner.Success()
}

// DownloadPublishedChart loads a specific chart version from a remote repo
func DownloadPublishedChart(chart types.ZarfChart, destination string) {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from repo %s", chart.Name, chart.Version, chart.Url)
	defer spinner.Stop()

	// Set up the helm pull config
	pull := action.NewPull()
	pull.Settings = cli.New()

	// Set up the chart chartDownloader
	chartDownloader := downloader.ChartDownloader{
		Out:     spinner,
		Verify:  downloader.VerifyNever,
		Getters: getter.All(pull.Settings),
	}

	// @todo: process OCI-based charts

	// Perform simple chart download
	chartURL, err := repo.FindChartInRepoURL(chart.Url, chart.Name, chart.Version, pull.CertFile, pull.KeyFile, pull.CaFile, getter.All(pull.Settings))
	if err != nil {
		spinner.Fatalf(err, "Unable to pull the helm chart")
	}

	// Download the file (we don't control what name helm creates here)
	saved, _, err := chartDownloader.DownloadTo(chartURL, pull.Version, destination)
	if err != nil {
		spinner.Fatalf(err, "Unable to download the helm chart")
	}

	// Ensure the name is consistent for deployments
	destinationTarball := StandardName(destination, chart) + ".tgz"
	err = os.Rename(saved, destinationTarball)
	if err != nil {
		spinner.Fatalf(err, "Unable to save the chart tarball")
	}

	spinner.Success()
}
