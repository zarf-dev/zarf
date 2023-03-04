// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/types/extensions"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxSrcCtrl "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Default location for pulling Big Bang.
const (
	bb     = "bigbang"
	bbRepo = "https://repo1.dso.mil/big-bang/bigbang.git"
)

var tenMins = metav1.Duration{
	Duration: 10 * time.Minute,
}

// Run Mutates a component that should deploy Big Bang to a set of manifests
// that contain the flux deployment of Big Bang
func Run(tmpPaths types.ComponentPaths, c types.ZarfComponent) (types.ZarfComponent, error) {
	var err error
	if err := utils.CreateDirectory(tmpPaths.Temp, 0700); err != nil {
		return c, fmt.Errorf("unable to component temp directory: %w", err)
	}

	cfg := c.Extensions.BigBang
	manifests := []types.ZarfManifest{}

	// Make sure the version is valid.
	if !isValidVersion(cfg.Version) {
		return c, fmt.Errorf("invalid Big Bang version: %s, must be at least 1.53.0", cfg.Version)
	}

	// Print the banner for Big Bang.
	printBanner()

	// If no repo is provided, use the default.
	if cfg.Repo == "" {
		cfg.Repo = bbRepo
	}

	// By default, we want to deploy flux.
	if !cfg.SkipFlux {
		fluxManifest, images, err := getFlux(tmpPaths.Temp, cfg)
		if err != nil {
			return c, err
		}

		// Add the flux manifests to the list of manifests to be pulled down by Zarf.
		manifests = append(manifests, fluxManifest)

		// Add the images to the list of images to be pulled down by Zarf.
		c.Images = append(c.Images, images...)
	}

	// Configure helm to pull down the Big Bang chart.
	helmCfg := helm.Helm{
		Chart: types.ZarfChart{
			Name:        bb,
			Namespace:   bb,
			URL:         cfg.Repo,
			Version:     cfg.Version,
			ValuesFiles: cfg.ValuesFiles,
			GitPath:     "./chart",
		},
		Cfg: &types.PackagerConfig{
			State: types.ZarfState{},
		},
		BasePath: tmpPaths.Temp,
	}

	// Download the chart from Git and save it to a temporary directory.
	chartPath := path.Join(tmpPaths.Temp, bb)
	helmCfg.ChartLoadOverride, err = helmCfg.PackageChartFromGit(chartPath)
	if err != nil {
		return c, fmt.Errorf("unable to download Big Bang Chart: %w", err)
	}

	// Template the chart so we can see what GitRepositories are being referenced in the
	// manifests created with the provided Helm.
	template, err := helmCfg.TemplateChart()
	if err != nil {
		return c, fmt.Errorf("unable to template Big Bang Chart: %w", err)
	}

	// Add the Big Bang repo to the list of repos to be pulled down by Zarf.
	bbRepo := fmt.Sprintf("%s@%s", cfg.Repo, cfg.Version)
	c.Repos = append(c.Repos, bbRepo)

	// Parse the template for GitRepository objects and add them to the list of repos to be pulled down by Zarf.
	urls, hrDependencies := findBBResources(template)
	c.Repos = append(c.Repos, urls...)

	// Generate a list of HelmReleases that need to be deployed in order.
	helmReleases, err := utils.SortDependencies(hrDependencies)
	if err != nil {
		return c, fmt.Errorf("unable to sort Big Bang HelmReleases: %w", err)
	}

	tenMinsInSecs := 10 * 60

	// Add wait actions for each of the helm releases in generally the order they should be deployed.
	for _, hr := range helmReleases {
		action := types.ZarfComponentAction{
			Description:     fmt.Sprintf("Big Bang Helm Release `%s` to be ready", hr),
			MaxTotalSeconds: &tenMinsInSecs,
			Wait: &types.ZarfComponentActionWait{
				Cluster: &types.ZarfComponentActionWaitCluster{
					Kind:       "HelmRelease",
					Identifier: hr,
					Namespace:  bb,
					Condition:  "ready",
				},
			},
		}

		// In Big Bang the metrics-server is a special case that only deploy if needed.
		// The check it, we need to look for the APIService to be exist instead of the HelmRelease, which
		// may not ever be created. See links below for more details.
		// https://repo1.dso.mil/big-bang/bigbang/-/blob/1.53.0/chart/templates/metrics-server/helmrelease.yaml
		if hr == "metrics-server" {
			action.Description = "K8s metric server to exist or be deployed by Big Bang"
			action.Wait.Cluster = &types.ZarfComponentActionWaitCluster{
				Kind: "APIService",
				// https://github.com/kubernetes-sigs/metrics-server#compatibility-matrix
				Identifier: "v1beta1.metrics.k8s.io",
			}
		}

		c.Actions.OnDeploy.OnSuccess = append(c.Actions.OnDeploy.OnSuccess, action)
	}

	t := true
	failureGeneral := []string{
		"get nodes -o wide",
		"get hr -n bigbang",
		"get gitrepo -n bigbang",
		"get pods -A",
	}
	failureDebug := []string{
		"describe hr -n bigbang",
		"describe gitrepo -n bigbang",
		"describe pods -A",
		"describe nodes",
		"get events -A",
	}

	// Add onFailure actions with additional troubleshooting information.
	for _, cmd := range failureGeneral {
		c.Actions.OnDeploy.OnFailure = append(c.Actions.OnDeploy.OnFailure, types.ZarfComponentAction{
			Cmd: fmt.Sprintf("./zarf tools kubectl %s", cmd),
		})
	}

	for _, cmd := range failureDebug {
		c.Actions.OnDeploy.OnFailure = append(c.Actions.OnDeploy.OnFailure, types.ZarfComponentAction{
			Mute:        &t,
			Description: "Storing debug information to the log for troubleshooting.",
			Cmd:         fmt.Sprintf("./zarf tools kubectl %s", cmd),
		})
	}

	// Add a pre-remove action to suspend the Big Bang HelmReleases to prevent reconciliation during removal.
	c.Actions.OnRemove.Before = append(c.Actions.OnRemove.Before, types.ZarfComponentAction{
		Description: "Suspend Big Bang HelmReleases to prevent reconciliation during removal.",
		Cmd:         `./zarf tools kubectl patch helmrelease -n bigbang bigbang --type=merge -p '{"spec":{"suspend":true}}'`,
	})

	// Select the images needed to support the repos for this configuration of Big Bang.
	for _, r := range c.Repos {
		images, err := helm.FindImagesForChartRepo(r, "chart")
		if err != nil {
			return c, fmt.Errorf("unable to find images for chart repo: %w", err)
		}

		c.Images = append(c.Images, images...)
	}

	// Make sure the list of images is unique.
	c.Images = utils.Unique(c.Images)

	// Create the flux wrapper around Big Bang for deployment.
	manifest, err := addBigBangManifests(tmpPaths.Temp, cfg)
	if err != nil {
		return c, err
	}

	// Add the Big Bang manifests to the list of manifests to be pulled down by Zarf.
	manifests = append(manifests, manifest)

	// Prepend the Big Bang manifests to the list of manifests to be pulled down by Zarf.
	// This is done so that the Big Bang manifests are deployed first.
	c.Manifests = append(manifests, c.Manifests...)

	return c, nil
}

// isValidVersion check if the version is 1.53.0 or greater.
func isValidVersion(version string) bool {
	// Split the version string into its major, minor, and patch components
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}

	// Parse the major and minor components as integers.
	// Ignore errors because we are checking the values later.
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])

	// This extension requires BB 1.53.0 or greater.
	// @todo: This should be updated to 1.54.0 when 1.55.0 is released.
	return major >= 1 && minor >= 53
}

// findBBResources takes a list of yaml objects (as a string) and
// parses it for GitRepository objects that it then parses
// to return the list of git repos and tags needed.
func findBBResources(t string) (urls []string, dependencies []utils.DependsOn) {
	// Break the template into separate resources.
	yamls, _ := utils.SplitYAMLToString([]byte(t))

	for _, y := range yamls {
		var (
			h fluxHelmCtrl.HelmRelease
			s fluxSrcCtrl.GitRepository
		)

		if err := yaml.Unmarshal([]byte(y), &h); err != nil {
			continue
		}

		// If the resource is a HelmRelease, parse it for the dependencies.
		if h.Kind == fluxHelmCtrl.HelmReleaseKind {
			var deps []string
			for _, d := range h.Spec.DependsOn {
				deps = append(deps, d.Name)
			}
			dependencies = append(dependencies, utils.DependsOn{
				Name:         h.ObjectMeta.Name,
				Dependencies: deps,
			})

			// Skip the rest as this is not a GitRepository.
			continue
		}

		if err := yaml.Unmarshal([]byte(y), &s); err != nil {
			continue
		}

		// If the resource is a GitRepository, parse it for the URL and tag.
		if s.Kind == fluxSrcCtrl.GitRepositoryKind && s.Spec.URL != "" {
			ref := "master"

			switch {
			case s.Spec.Reference.Commit != "":
				ref = s.Spec.Reference.Commit

			case s.Spec.Reference.SemVer != "":
				ref = s.Spec.Reference.SemVer

			case s.Spec.Reference.Tag != "":
				ref = s.Spec.Reference.Tag

			case s.Spec.Reference.Branch != "":
				ref = s.Spec.Reference.Branch
			}

			// Append the URL and tag to the list.
			urls = append(urls, fmt.Sprintf("%s@%s", s.Spec.URL, ref))
		}
	}

	return urls, dependencies
}

// addBigBangManifests creates the manifests component for deploying Big Bang.
func addBigBangManifests(manifestDir string, cfg *extensions.BigBang) (types.ZarfManifest, error) {
	// Create a manifest component that we add to the zarf package for bigbang.
	manifest := types.ZarfManifest{
		Name:      bb,
		Namespace: bb,
	}

	// Helper function to marshal and write a manifest and add it to the component.
	addManifest := func(name string, data any) error {
		path := path.Join(manifestDir, name)
		out, err := yaml.Marshal(data)
		if err != nil {
			return err
		}

		if err := utils.WriteFile(path, out); err != nil {
			return err
		}

		manifest.Files = append(manifest.Files, path)
		return nil
	}

	// Create the GitRepository manifest.
	if err := addManifest("bb-ext-gitrepository.yaml", manifestGitRepo(cfg)); err != nil {
		return manifest, err
	}

	// Create the zarf-credentials secret manifest.
	if err := addManifest("bb-ext-zarf-credentials.yaml", manifestZarfCredentials()); err != nil {
		return manifest, err
	}

	// Create the list of values manifests starting with zarf-credentials.
	hrValues := []fluxHelmCtrl.ValuesReference{{
		Kind: "Secret",
		Name: "zarf-credentials",
	}}

	// Loop through the valuesFrom list and create a manifest for each.
	for _, path := range cfg.ValuesFiles {
		data, err := manifestValuesFile(path)
		if err != nil {
			return manifest, err
		}

		path := fmt.Sprintf("%s.yaml", data.Name)
		if err := addManifest(path, data); err != nil {
			return manifest, err
		}

		// Add it to the list of valuesFrom for the HelmRelease
		hrValues = append(hrValues, fluxHelmCtrl.ValuesReference{
			Kind: "Secret",
			Name: data.Name,
		})
	}

	if err := addManifest("bb-ext-helmrepository.yaml", manifestHelmRelease(hrValues)); err != nil {
		return manifest, err
	}

	return manifest, nil
}
