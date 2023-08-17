// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// PackageChart creates a chart archive from a path to a chart on the host os and builds chart dependencies
func (h *HelmCfg) PackageChart(destination string) error {
	if len(h.chart.URL) > 0 {
		url, refPlain, err := transform.GitURLSplitRef(h.chart.URL)
		// check if the chart is a git url with a ref (if an error is returned url will be empty)
		isGitURL := strings.HasSuffix(url, ".git")
		if err != nil {
			message.Debugf("unable to parse the url, continuing with %s", h.chart.URL)
		}

		if isGitURL {
			// if it is a git url append chart version as if its a tag
			if refPlain == "" {
				h.chart.URL = fmt.Sprintf("%s@%s", h.chart.URL, h.chart.Version)
			}

			_, err = h.PackageChartFromGit(destination)

			if err != nil {
				return fmt.Errorf("error creating chart archive, unable to pull the chart from git: %s", err.Error())
			}
		} else {
			h.DownloadPublishedChart(destination)
		}

	} else {
		_, err := h.PackageChartFromLocalFiles(destination)
		if err != nil {
			return fmt.Errorf("error creating chart archive, unable to package the chart: %s", err.Error())
		}
	}
	return nil
}

// PackageChartFromLocalFiles creates a chart archive from a path to a chart on the host os.
func (h *HelmCfg) PackageChartFromLocalFiles(destination string) (string, error) {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s", h.chart.Name, h.chart.Version, h.chart.LocalPath)
	defer spinner.Stop()

	// Validate the chart
	_, err := loader.LoadDir(h.chart.LocalPath)
	if err != nil {
		spinner.Errorf(err, "Validation failed for chart from %s (%s)", h.chart.LocalPath, err.Error())
		return "", err
	}

	err = h.buildChartDependencies(spinner)
	if err != nil {
		spinner.Errorf(err, "Unable to build dependencies for the chart: %s", err.Error())
		return "", err
	}

	client := action.NewPackage()

	client.Destination = destination
	path, err := client.Run(h.chart.LocalPath, nil)

	if err != nil {
		spinner.Errorf(err, "Helm is unable to save the archive and create the package %s", path)
		return "", err
	}

	spinner.Success()

	return path, nil
}

// PackageChartFromGit is a special implementation of chart archiving that supports the https://p1.dso.mil/#/products/big-bang/ model.
func (h *HelmCfg) PackageChartFromGit(destination string) (string, error) {
	spinner := message.NewProgressSpinner("Processing helm chart %s", h.chart.Name)
	defer spinner.Stop()

	// Retrieve the repo containing the chart
	gitPath, err := h.DownloadChartFromGitToTemp(spinner)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(gitPath)

	// Set the directory for the chart and package it
	h.chart.LocalPath = filepath.Join(gitPath, h.chart.GitPath)
	return h.PackageChartFromLocalFiles(destination)
}

// DownloadPublishedChart loads a specific chart version from a remote repo.
func (h *HelmCfg) DownloadPublishedChart(destination string) {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from repo %s", h.chart.Name, h.chart.Version, h.chart.URL)
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
	if registry.IsOCI(h.chart.URL) {
		regClient, err = registry.NewClient(registry.ClientOptEnableCache(true))
		if err != nil {
			spinner.Fatalf(err, "Unable to create a new registry client")
		}
		chartURL = h.chart.URL
		// Explicitly set the pull version for OCI
		pull.Version = h.chart.Version
	} else {
		// Perform simple chart download
		chartURL, err = repo.FindChartInRepoURL(h.chart.URL, h.chart.Name, h.chart.Version, pull.CertFile, pull.KeyFile, pull.CaFile, getter.All(pull.Settings))
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
		spinner.Fatalf(err, "Validation failed for chart %s (%s)", h.chart.Name, err.Error())
	}

	// Ensure the name is consistent for deployments
	destinationTarball := StandardName(destination, h.chart) + ".tgz"
	err = os.Rename(saved, destinationTarball)
	if err != nil {
		spinner.Fatalf(err, "Unable to save the chart tarball")
	}

	spinner.Success()
}

// DownloadChartFromGitToTemp downloads a chart from git into a temp directory
func (h *HelmCfg) DownloadChartFromGitToTemp(spinner *message.Spinner) (string, error) {
	// Create the Git configuration and download the repo
	gitCfg := git.NewWithSpinner(types.GitServerInfo{}, spinner)

	// Download the git repo to a temporary directory
	err := gitCfg.DownloadRepoToTemp(h.chart.URL)
	if err != nil {
		spinner.Errorf(err, "Unable to download the git repo %s", h.chart.URL)
		return "", err
	}

	return gitCfg.GitPath, nil
}

// buildChartDependencies builds the helm chart dependencies
func (h *HelmCfg) buildChartDependencies(spinner *message.Spinner) error {
	regClient, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		spinner.Fatalf(err, "Unable to create a new registry client")
	}
	h.settings = cli.New()
	man := &downloader.Manager{
		Out:            os.Stdout,
		ChartPath:      h.chart.LocalPath,
		Getters:        getter.All(h.settings),
		RegistryClient: regClient,

		RepositoryConfig: h.settings.RepositoryConfig,
		RepositoryCache:  h.settings.RepositoryCache,
		Debug:            false,
	}
	// Verify the chart
	man.Verify = downloader.VerifyIfPossible

	// Build the deps from the helm chart
	err = man.Build()
	if e, ok := err.(downloader.ErrRepoNotFound); ok {
		return fmt.Errorf("%s. Please add the missing repo via 'helm repo add <name> <url>'", e.Error())
	} else if err != nil {
		return err
	}

	return nil
}
