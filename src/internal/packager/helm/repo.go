package helm

import (
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// CreateChartFromLocalFiles creates a chart archive from a path to a chart on the host os
func CreateChartFromLocalFiles(chart types.ZarfChart, destination string) string {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s", chart.Name, chart.Version, chart.LocalPath)
	defer spinner.Stop()

	// Validate the chart
	_, err := loader.LoadDir(chart.LocalPath)
	if err != nil {
		spinner.Fatalf(err, "Validation failed for chart from %s (%s)", chart.LocalPath, err.Error())
	}

	client := action.NewPackage()

	client.Destination = destination
	path, err := client.Run(chart.LocalPath, nil)

	if err != nil {
		spinner.Fatalf(err, "Helm is unable to save the archive and create the package %s", path)
	}

	spinner.Success()

	return path
}

// DownloadChartFromGit is a special implementation of chart downloads that support the https://p1.dso.mil/#/products/big-bang/ model
func (h *Helm) DownloadChartFromGit(chart types.ZarfChart, destination string) string {
	spinner := message.NewProgressSpinner("Processing helm chart %s", chart.Name)
	defer spinner.Stop()

	client := action.NewPackage()

	// Get the git repo
	gitCfg := git.NewWithSpinner(h.Cfg.State.GitServer, spinner)
	gitCfg := git.Git{
		Server: h.Cfg.State.GitServer,
		Spinner: spinner,
		GitPath: ,
	}
	tempPath := gitCfg.DownloadRepoToTemp(chart.Url)

	// Switch to the correct tag
	gitCfg.CheckoutTag(tempPath, chart.Version)

	// Validate the chart
	_, err := loader.LoadDir(filepath.Join(tempPath, chart.GitPath))
	if err != nil {
		spinner.Fatalf(err, "Validation failed for chart %s (%s)", chart.Name, err.Error())
	}

	// Tell helm where to save the archive and create the package
	client.Destination = destination
	name, err := client.Run(filepath.Join(tempPath, chart.GitPath), nil)

	if err != nil {
		spinner.Fatalf(err, "Helm is unable to save the archive and create the package %s", name)
	}

	_ = os.RemoveAll(tempPath)
	spinner.Success()

	return name
}

// DownloadPublishedChart loads a specific chart version from a remote repo
func (h *Helm) DownloadPublishedChart(chart types.ZarfChart, destination string) {
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

	// Validate the chart
	_, err = loader.LoadFile(saved)
	if err != nil {
		spinner.Fatalf(err, "Validation failed for chart %s (%s)", chart.Name, err.Error())
	}

	// Ensure the name is consistent for deployments
	destinationTarball := StandardName(destination, chart) + ".tgz"
	err = os.Rename(saved, destinationTarball)
	if err != nil {
		spinner.Fatalf(err, "Unable to save the chart tarball")
	}

	spinner.Success()
}
