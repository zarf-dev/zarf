// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"helm.sh/helm/v4/pkg/action"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/downloader"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
	repov1 "helm.sh/helm/v4/pkg/repo/v1"

	retry "github.com/avast/retry-go/v4"
	orasRegistry "oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// negotiateChartPlainHTTP decides the transport scheme for an OCI chart or chart
// dependency host discovered in package data (not named on the command line).
// remoteOptions.PlainHTTP gates whether to probe at all: unset skips the network
// call and defaults to HTTPS; set verifies this specific host rather than forcing
// plain HTTP onto it directly.
func negotiateChartPlainHTTP(ctx context.Context, ociURL string, remoteOptions types.RemoteOptions) (bool, error) {
	if !remoteOptions.PlainHTTP {
		return false, nil
	}
	ref, err := orasRegistry.ParseReference(strings.TrimPrefix(ociURL, helpers.OCIURLPrefix))
	if err != nil {
		return false, fmt.Errorf("unable to parse chart url %q: %w", ociURL, err)
	}
	plainHTTP, err := ocischeme.From(ctx).UsePlainHTTP(ctx, ref.Registry, ocischeme.ProbeOptions{InsecureSkipTLSVerify: remoteOptions.InsecureSkipTLSVerify})
	if err != nil {
		return false, fmt.Errorf("unable to reach chart registry for %q: %w", ociURL, err)
	}
	return plainHTTP, nil
}

// negotiateLoadedChartDependenciesPlainHTTP negotiates a chart's OCI-referenced
// dependency hosts (see negotiateChartPlainHTTP). Charts with no OCI dependencies
// return HTTPS without probing. Dependencies spanning hosts that disagree on scheme
// default to HTTPS rather than guessing.
func negotiateLoadedChartDependenciesPlainHTTP(ctx context.Context, chartName string, dependencies []*chartv2.Dependency, remoteOptions types.RemoteOptions) (bool, error) {
	var decided bool
	var decidedSet bool
	for _, dep := range dependencies {
		if !registry.IsOCI(dep.Repository) {
			continue
		}
		plainHTTP, err := negotiateChartPlainHTTP(ctx, dep.Repository, remoteOptions)
		if err != nil {
			return false, err
		}
		if !decidedSet {
			decided, decidedSet = plainHTTP, true
			continue
		}
		if decided != plainHTTP {
			logger.From(ctx).Debug("chart's OCI dependencies span hosts that disagree on plain-http vs HTTPS; defaulting to HTTPS", "chart", chartName)
			return false, nil
		}
	}
	return decided, nil
}

// PackageChart creates a chart archive from a path to a chart on the host os and builds chart dependencies
func PackageChart(ctx context.Context, chart v1alpha1.ZarfChart, chartPath, valuesPath string, cachePath string, remoteOptions types.RemoteOptions) error {
	if len(chart.URL) > 0 {
		url, refPlain, err := transform.GitURLSplitRef(chart.URL)
		// check if the chart is a git url with a ref (if an error is returned url will be empty)
		isGitURL := strings.HasSuffix(url, ".git")
		if err != nil {
			logger.From(ctx).Debug("unable to parse the url, continuing", "url", chart.URL)
		}

		if isGitURL {
			// if it is a git url append chart version as if its a tag
			if refPlain == "" {
				chart.URL = fmt.Sprintf("%s@%s", chart.URL, chart.Version)
			}

			err = PackageChartFromGit(ctx, chart, chartPath, valuesPath, cachePath, remoteOptions)
			if err != nil {
				return fmt.Errorf("unable to pull the chart %q from git: %w", chart.Name, err)
			}
		} else {
			err = DownloadPublishedChart(ctx, chart, chartPath, valuesPath, cachePath, remoteOptions)
			if err != nil {
				return fmt.Errorf("unable to download the published chart %q: %w", chart.Name, err)
			}
		}
	} else {
		err := PackageChartFromLocalFiles(ctx, chart, chartPath, valuesPath, cachePath, remoteOptions)
		if err != nil {
			return fmt.Errorf("unable to package the %q chart: %w", chart.Name, err)
		}
	}
	return nil
}

// PackageChartFromLocalFiles creates a chart archive from a path to a chart on the host os.
func PackageChartFromLocalFiles(ctx context.Context, chart v1alpha1.ZarfChart, chartPath string, valuesPath string, cachePath string, remoteOptions types.RemoteOptions) error {
	l := logger.From(ctx)
	l.Info("processing local helm chart",
		"name", chart.Name,
		"version", chart.Version,
		"path", chart.LocalPath,
	)

	// Load and validate the chart
	cl, parsed, err := loadAndValidateChart(chart.LocalPath)
	if err != nil {
		return err
	}

	// Handle the chart directory or tarball
	var saved string
	temp := filepath.Join(chartPath, "temp")
	if _, ok := cl.(loader.DirLoader); ok {
		err = buildChartDependencies(ctx, chart, cachePath, parsed.Metadata.Dependencies, remoteOptions)
		if err != nil {
			return fmt.Errorf("unable to build dependencies for the chart: %w", err)
		}

		client := action.NewPackage()

		client.Destination = temp
		saved, err = client.Run(chart.LocalPath, nil)
	} else {
		saved = filepath.Join(temp, filepath.Base(chart.LocalPath))
		err = helpers.CreatePathAndCopy(chart.LocalPath, saved)
	}
	defer func(l *slog.Logger) {
		err := os.RemoveAll(temp)
		if err != nil {
			l.Error(err.Error())
		}
	}(l)

	if err != nil {
		return fmt.Errorf("unable to save the archive and create the package %s: %w", saved, err)
	}

	// Finalize the chart
	err = finalizeChartPackage(ctx, chart, chartPath, valuesPath, saved)
	if err != nil {
		return err
	}

	l.Debug("done processing local helm chart",
		"name", chart.Name,
		"version", chart.Version,
		"path", chart.LocalPath,
	)
	return nil
}

// PackageChartFromGit is a special implementation of chart archiving that supports the https://p1.dso.mil/#/products/big-bang/ model.
func PackageChartFromGit(ctx context.Context, chart v1alpha1.ZarfChart, chartPath, valuesPath, cachePath string, remoteOptions types.RemoteOptions) error {
	l := logger.From(ctx)
	l.Info("processing Helm chart", "name", chart.Name)

	// Retrieve the repo containing the chart
	gitPath, err := DownloadChartFromGitToTemp(ctx, chart.URL)
	if err != nil {
		return err
	}
	defer func(l *slog.Logger) {
		if err := os.RemoveAll(gitPath); err != nil {
			l.Error(err.Error())
		}
	}(l)

	// Set the directory for the chart and package it
	chart.LocalPath = filepath.Join(gitPath, chart.GitPath)
	return PackageChartFromLocalFiles(ctx, chart, chartPath, valuesPath, cachePath, remoteOptions)
}

// DownloadPublishedChart loads a specific chart version from a remote repo.
func DownloadPublishedChart(ctx context.Context, chart v1alpha1.ZarfChart, chartPath, valuesPath, cachePath string, remoteOptions types.RemoteOptions) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Info("processing Helm chart",
		"name", chart.Name,
		"version", chart.Version,
		"repo", chart.URL,
	)

	// Set up the helm pull config
	pull := action.NewPull()
	pull.Settings = cli.New()

	var (
		regClient *registry.Client
		chartURL  string
		err       error
		// plainHTTP is always negotiated for an OCI chart host (never taken from
		// remoteOptions.PlainHTTP directly): the host was discovered by reading package
		// data or a repo index, not named explicitly on this command line, so the global
		// --plain-http flag is not necessarily meant for it.
		plainHTTP bool
	)
	repoFile, err := repov1.LoadFile(pull.Settings.RepositoryConfig)

	// Not returning the error here since the repo file is only needed if we are pulling from a repo that requires authentication
	if err != nil {
		l.Debug("unable to load the repo file",
			"path", pull.Settings.RepositoryConfig,
			"error", err.Error(),
		)
	}

	var username string
	var password string

	// Handle OCI registries
	if registry.IsOCI(chart.URL) {
		chartURL = chart.URL
		// Explicitly set the pull version for OCI
		pull.Version = chart.Version
	} else {
		chartName := chart.Name
		if chart.RepoName != "" {
			chartName = chart.RepoName
		}

		if repoFile != nil {
			// TODO: @AustinAbro321 Currently this selects the last repo with the same url
			// We should introduce a new field in zarf to allow users to specify the local repo they want
			for _, repo := range repoFile.Repositories {
				if repo.URL == chart.URL {
					username = repo.Username
					password = repo.Password
				}
			}
		}

		chartURL, err = repov1.FindChartInRepoURL(
			chart.URL,
			chartName,
			getter.All(pull.Settings),
			repov1.WithChartVersion(chart.Version),
			repov1.WithUsernamePassword(username, password),
			repov1.WithClientTLS(pull.CertFile, pull.KeyFile, pull.CaFile),
			repov1.WithInsecureSkipTLSVerify(remoteOptions.InsecureSkipTLSVerify),
		)
		if err != nil {
			return fmt.Errorf("unable to pull the helm chart: %w", err)
		}
	}

	// chartURL is OCI either when given directly or when a classic Helm repo
	// index redirects to an OCI reference
	if registry.IsOCI(chartURL) {
		plainHTTP, err = negotiateChartPlainHTTP(ctx, chartURL, remoteOptions)
		if err != nil {
			return err
		}
		clientOpts := []registry.ClientOption{registry.ClientOptEnableCache(true)}
		if plainHTTP {
			clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
		}
		regClient, err = registry.NewClient(clientOpts...)
		if err != nil {
			return fmt.Errorf("unable to create the new registry client: %w", err)
		}
	}

	contentCache := filepath.Join(cachePath, contentCachePath)

	// Set up the chart chartDownloader
	chartDownloader := downloader.ChartDownloader{
		Out:            io.Discard,
		RegistryClient: regClient,
		ContentCache:   contentCache,
		// TODO: Further research this with regular/OCI charts
		Verify:  downloader.VerifyNever,
		Getters: getter.All(pull.Settings),
		Options: []getter.Option{
			// plainHTTP is negotiated only in the OCI branch above and stays false for
			// a traditional repo; Helm's http/https getter (unlike its OCI getter)
			// never reads this option, taking its scheme from chartURL instead.
			getter.WithPlainHTTP(plainHTTP),
			getter.WithInsecureSkipVerifyTLS(remoteOptions.InsecureSkipTLSVerify),
			getter.WithBasicAuth(username, password),
		},
	}

	var saved string
	err = retry.Do(
		func() error {
			var downloadErr error
			saved, _, downloadErr = chartDownloader.DownloadToCache(chartURL, pull.Version)
			return downloadErr
		},
		retry.Attempts(uint(config.ZarfDefaultRetries)),
		retry.Delay(config.ZarfDefaultRetryDelay),
		retry.MaxDelay(config.ZarfDefaultRetryMaxDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			if config.ZarfDefaultRetries > 1 && n+1 < uint(config.ZarfDefaultRetries) {
				l.Warn("retrying chart download",
					"attempt", n+1,
					"maxAttempts", config.ZarfDefaultRetries,
					"chart", chart.Name,
					"error", err,
				)
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("unable to download the helm chart: %w", err)
	}

	// Validate the chart
	_, _, err = loadAndValidateChart(saved)
	if err != nil {
		return err
	}

	// Finalize the chart
	err = finalizeChartPackage(ctx, chart, chartPath, valuesPath, saved)
	if err != nil {
		return err
	}

	l.Debug("done downloading helm chart",
		"name", chart.Name,
		"version", chart.Version,
		"repo", chart.URL,
		"duration", time.Since(start),
	)
	return nil
}

// DownloadChartFromGitToTemp downloads a chart from git into a temp directory
func DownloadChartFromGitToTemp(ctx context.Context, url string) (string, error) {
	path, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", fmt.Errorf("unable to create tmpdir: %w", err)
	}
	repository, err := git.Clone(ctx, path, url, true)
	if err != nil {
		return "", err
	}
	return repository.Path(), nil
}

func finalizeChartPackage(ctx context.Context, chart v1alpha1.ZarfChart, chartPath, valuesPath, saved string) error {
	// Ensure the name is consistent for deployments
	destinationTarball := StandardName(chartPath, chart) + ".tgz"
	err := helpers.CreatePathAndCopy(saved, destinationTarball)
	if err != nil {
		return fmt.Errorf("unable to save the final chart tarball: %w", err)
	}

	err = packageValues(ctx, chart, valuesPath)
	if err != nil {
		return fmt.Errorf("unable to process the values for the package: %w", err)
	}
	return nil
}

func packageValues(ctx context.Context, chart v1alpha1.ZarfChart, valuesPath string) error {
	for _, f := range GetChartValuesFiles(chart) {
		dst := StandardValuesName(valuesPath, chart, f.GlobalIdx)

		if helpers.IsURL(f.Source) {
			if err := utils.DownloadToFile(ctx, f.Source, dst); err != nil {
				return fmt.Errorf(lang.ErrDownloading, f.Source, err)
			}
		} else {
			if err := helpers.CreatePathAndCopy(f.Source, dst); err != nil {
				return fmt.Errorf("unable to copy chart values file %s: %w", f.Source, err)
			}
		}
	}

	return nil
}

// buildChartDependencies builds the helm chart dependencies. dependencies is the
// already-loaded chart's declared Chart.yaml dependencies; the caller has already
// loaded the chart to get here, so this avoids reloading it from disk.
func buildChartDependencies(ctx context.Context, chart v1alpha1.ZarfChart, cachePath string, dependencies []*chartv2.Dependency, remoteOptions types.RemoteOptions) error {
	l := logger.From(ctx)

	// negotiate the transport instead of forcing the global flag.
	plainHTTP, err := negotiateLoadedChartDependenciesPlainHTTP(ctx, chart.Name, dependencies, remoteOptions)
	if err != nil {
		return err
	}

	// Download and build the specified dependencies
	clientOpts := []registry.ClientOption{registry.ClientOptEnableCache(true)}
	if plainHTTP {
		clientOpts = append(clientOpts, registry.ClientOptPlainHTTP())
	}
	regClient, err := registry.NewClient(clientOpts...)
	if err != nil {
		return fmt.Errorf("unable to create a new registry client: %w", err)
	}

	settings := cli.New()

	contentCache := filepath.Join(cachePath, contentCachePath)

	man := &downloader.Manager{
		Out:            &logger.LogWriter{Logger: l, Level: logger.Debug},
		ContentCache:   contentCache,
		ChartPath:      chart.LocalPath,
		Getters:        getter.All(settings),
		RegistryClient: regClient,

		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		Debug:            false,
		Verify:           downloader.VerifyNever,
	}

	// Build the deps from the helm chart
	err = man.Build()
	var notFoundErr *downloader.ErrRepoNotFound
	if errors.As(err, &notFoundErr) {
		// If we encounter a repo not found error point the user to `zarf tools helm repo add`
		l.Warn("Error occurred", "error", notFoundErr.Error())
		l.Warn("Please add the missing repo(s) via the following:")
		for _, repository := range notFoundErr.Repos {
			l.Warn("$zarf tools helm repo add <your-repo-name>", "repository", repository)
		}
		return err
	}
	if err != nil {
		l.Info("$zarf tools helm dependency build --verify")
		l.Warn("unable to perform a rebuild of Helm dependencies", "error", err.Error())
		return err
	}
	return nil
}

func loadAndValidateChart(location string) (loader.ChartLoader, *chartv2.Chart, error) {
	// Validate the chart
	cl, err := loader.Loader(location)
	if err != nil {
		return cl, nil, fmt.Errorf("unable to load the chart from %s: %w", location, err)
	}

	chart, err := cl.Load()
	if err != nil {
		return cl, chart, fmt.Errorf("validation failed for chart from %s: %w", location, err)
	}

	return cl, chart, nil
}
