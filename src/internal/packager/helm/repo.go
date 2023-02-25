// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// PackageChartFromLocalFiles creates a chart archive from a path to a chart on the host os.
func (h *Helm) PackageChartFromLocalFiles(destination string) string {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s", h.Chart.Name, h.Chart.Version, h.Chart.LocalPath)
	defer spinner.Stop()

	// Validate the chart
	_, err := loader.LoadDir(h.Chart.LocalPath)
	if err != nil {
		spinner.Fatalf(err, "Validation failed for chart from %s (%s)", h.Chart.LocalPath, err.Error())
	}

	client := action.NewPackage()

	client.Destination = destination
	path, err := client.Run(h.Chart.LocalPath, nil)

	if err != nil {
		spinner.Fatalf(err, "Helm is unable to save the archive and create the package %s", path)
	}

	spinner.Success()

	return path
}

// PackageChartFromGit is a special implementation of chart archiving that supports the https://p1.dso.mil/#/products/big-bang/ model.
func (h *Helm) PackageChartFromGit(destination string) string {
	spinner := message.NewProgressSpinner("Processing helm chart %s", h.Chart.Name)
	defer spinner.Stop()

	client := action.NewPackage()

	tempPath, cleanup := h.downloadChartFromGitToTemp(spinner)
	defer cleanup()

	// Validate the chart
	if _, err := loader.LoadDir(tempPath); err != nil {
		spinner.Fatalf(err, "Validation failed for chart %s (%s)", h.Chart.Name, err.Error())
	}

	// Tell helm where to save the archive and create the package
	client.Destination = destination
	name, err := client.Run(tempPath, nil)
	if err != nil {
		spinner.Fatalf(err, "Helm is unable to save the archive and create the package %s", name)
	}

	spinner.Success()

	return name
}

// DownloadPublishedChart loads a specific chart version from a remote repo.
func (h *Helm) DownloadPublishedChart(destination string) {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from repo %s", h.Chart.Name, h.Chart.Version, h.Chart.URL)
	defer spinner.Stop()

	// Set up the helm pull config
	pull := action.NewPull()
	pull.Settings = cli.New()

	var (
		regClient *registry.Client
		chartURL  string
		err       error
	)

	// Handle OCI registries
	if registry.IsOCI(h.Chart.URL) {
		regClient, err = registry.NewClient(registry.ClientOptEnableCache(true))
		if err != nil {
			spinner.Fatalf(err, "Unable to create a new registry client")
		}
		chartURL = h.Chart.URL
	} else {
		// Perform simple chart download
		chartURL, err = repo.FindChartInRepoURL(h.Chart.URL, h.Chart.Name, h.Chart.Version, pull.CertFile, pull.KeyFile, pull.CaFile, getter.All(pull.Settings))
		if err != nil {
			spinner.Fatalf(err, "Unable to pull the helm chart")
		}
	}

	// Set up the chart chartDownloader
	chartDownloader := downloader.ChartDownloader{
		Out:            spinner,
		RegistryClient: regClient,
		// TODO: Further research this with regular/OCI charts
		Verify:  downloader.VerifyNever,
		Getters: getter.All(pull.Settings),
		Options: []getter.Option{
			getter.WithInsecureSkipVerifyTLS(config.CommonOptions.Insecure),
		},
	}

	// Download the file (we don't control what name helm creates here)
	saved, _, err := chartDownloader.DownloadTo(chartURL, pull.Version, destination)
	if err != nil {
		spinner.Fatalf(err, "Unable to download the helm chart")
	}

	// Validate the chart
	_, err = loader.LoadFile(saved)
	if err != nil {
		spinner.Fatalf(err, "Validation failed for chart %s (%s)", h.Chart.Name, err.Error())
	}

	// Ensure the name is consistent for deployments
	destinationTarball := StandardName(destination, h.Chart) + ".tgz"
	err = os.Rename(saved, destinationTarball)
	if err != nil {
		spinner.Fatalf(err, "Unable to save the chart tarball")
	}

	spinner.Success()
}

func (h *Helm) downloadChartFromGitToTemp(spinner *message.Spinner) (string, func()) {
	// Create the Git configuration and download the repo
	gitCfg := git.NewWithSpinner(h.Cfg.State.GitServer, spinner)

	tempPath, err := gitCfg.DownloadRepoToTemp(h.Chart.URL)
	if err != nil {
		spinner.Fatalf(err, "Unable to download the git repo %s", h.Chart.URL)
	}
	gitCfg.GitPath = tempPath

	// Switch to the correct tag as specified by the cart version
	if err = gitCfg.CheckoutTag(h.Chart.Version); err != nil {
		spinner.Fatalf(err, "Unable to download provided git refrence: %v@%v", h.Chart.URL, h.Chart.Version)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempPath)
	}

	return filepath.Join(tempPath, h.Chart.GitPath), cleanup
}
