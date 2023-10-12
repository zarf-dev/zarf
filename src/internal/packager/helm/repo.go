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
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/client-go/util/homedir"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// PackageChart creates a chart archive from a path to a chart on the host os and builds chart dependencies
func (h *Helm) PackageChart(destination string) error {
	if len(h.Chart.URL) > 0 {
		url, refPlain, err := transform.GitURLSplitRef(h.Chart.URL)
		// check if the chart is a git url with a ref (if an error is returned url will be empty)
		isGitURL := strings.HasSuffix(url, ".git")
		if err != nil {
			message.Debugf("unable to parse the url, continuing with %s", h.Chart.URL)
		}

		if isGitURL {
			// if it is a git url append chart version as if its a tag
			if refPlain == "" {
				h.Chart.URL = fmt.Sprintf("%s@%s", h.Chart.URL, h.Chart.Version)
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
func (h *Helm) PackageChartFromLocalFiles(destination string) (string, error) {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s", h.Chart.Name, h.Chart.Version, h.Chart.LocalPath)
	defer spinner.Stop()

	// Validate the chart
	cl, err := loader.Loader(h.Chart.LocalPath)
	if err != nil {
		spinner.Errorf(err, "Validation failed for chart from %s (%s)", h.Chart.LocalPath, err.Error())
		return "", err
	}

	var path string
	if _, ok := cl.(loader.DirLoader); ok {
		err = h.buildChartDependencies(spinner)
		if err != nil {
			spinner.Errorf(err, "Unable to build dependencies for the chart: %s", err.Error())
			return "", err
		}

		client := action.NewPackage()

		client.Destination = destination
		path, err = client.Run(h.Chart.LocalPath, nil)
	} else {
		path = filepath.Join(destination, filepath.Base(h.Chart.LocalPath))
		err = utils.CreatePathAndCopy(h.Chart.LocalPath, path)
	}

	if err != nil {
		spinner.Errorf(err, "Helm is unable to save the archive and create the package %s", path)
		return "", err
	}

	spinner.Success()

	return path, nil
}

// PackageChartFromGit is a special implementation of chart archiving that supports the https://p1.dso.mil/#/products/big-bang/ model.
func (h *Helm) PackageChartFromGit(destination string) (string, error) {
	spinner := message.NewProgressSpinner("Processing helm chart %s", h.Chart.Name)
	defer spinner.Stop()

	// Retrieve the repo containing the chart
	gitPath, err := h.DownloadChartFromGitToTemp(spinner)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(gitPath)

	// Set the directory for the chart and package it
	h.Chart.LocalPath = filepath.Join(gitPath, h.Chart.GitPath)
	return h.PackageChartFromLocalFiles(destination)
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
		// Explicitly set the pull version for OCI
		pull.Version = h.Chart.Version
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

// DownloadChartFromGitToTemp downloads a chart from git into a temp directory
func (h *Helm) DownloadChartFromGitToTemp(spinner *message.Spinner) (string, error) {
	// Create the Git configuration and download the repo
	gitCfg := git.NewWithSpinner(types.GitServerInfo{}, spinner)

	// Download the git repo to a temporary directory
	err := gitCfg.DownloadRepoToTemp(h.Chart.URL)
	if err != nil {
		spinner.Errorf(err, "Unable to download the git repo %s", h.Chart.URL)
		return "", err
	}

	return gitCfg.GitPath, nil
}

// buildChartDependencies builds the helm chart dependencies
func (h *Helm) buildChartDependencies(spinner *message.Spinner) error {
	// Download and build the specified dependencies
	regClient, err := registry.NewClient(registry.ClientOptEnableCache(true))
	if err != nil {
		spinner.Fatalf(err, "Unable to create a new registry client")
	}

	h.Settings = cli.New()
	defaultKeyring := filepath.Join(homedir.HomeDir(), ".gnupg", "pubring.gpg")
	if v, ok := os.LookupEnv("GNUPGHOME"); ok {
		defaultKeyring = filepath.Join(v, "pubring.gpg")
	}

	man := &downloader.Manager{
		Out:            &message.DebugWriter{},
		ChartPath:      h.Chart.LocalPath,
		Getters:        getter.All(h.Settings),
		RegistryClient: regClient,

		RepositoryConfig: h.Settings.RepositoryConfig,
		RepositoryCache:  h.Settings.RepositoryCache,
		Debug:            false,
		Verify:           downloader.VerifyIfPossible,
		Keyring:          defaultKeyring,
	}

	// Build the deps from the helm chart
	err = man.Build()
	if e, ok := err.(downloader.ErrRepoNotFound); ok {
		// If we encounter a repo not found error point the user to `zarf tools helm repo add`
		message.Warnf("%s. Please add the missing repo(s) via the following:", e.Error())
		for _, repository := range e.Repos {
			message.ZarfCommand(fmt.Sprintf("tools helm repo add <your-repo-name> %s", repository))
		}
	} else if err != nil {
		// Warn the user of any issues but don't fail - any actual issues will cause a fail during packaging (e.g. the charts we are building may exist already, we just can't get updates)
		message.ZarfCommand("tools helm dependency build --verify")
		message.Warnf("Unable to perform a rebuild of Helm dependencies: %s", err.Error())
	}

	return nil
}
