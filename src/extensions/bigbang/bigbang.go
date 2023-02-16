// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing BigBang and Flux
package bigbang

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/types/extensions"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxSrcCtrl "github.com/fluxcd/source-controller/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// Default location for pulling BigBang.
const (
	_BB      = "bigbang"
	_BB_REPO = "https://repo1.dso.mil/big-bang/bigbang.git"
)

var tenMins = metav1.Duration{
	Duration: 10 * time.Minute,
}

// MutateBigbangComponent Mutates a component that should deploy BigBang to a set of manifests
// that contain the flux deployment of BigBang
func MutateBigbangComponent(componentPath types.ComponentPaths, component types.ZarfComponent) (types.ZarfComponent, error) {
	_ = utils.CreateDirectory(componentPath.Charts, 0700)
	_ = utils.CreateDirectory(componentPath.Manifests, 0700)

	repos := make([]string, 0)
	cfg := component.Extensions.BigBang

	// use the default repo unless overridden
	if cfg.Repo == "" {
		repos = append(repos, _BB_REPO)
		cfg.Repo = repos[0]
	} else {
		repos = append(repos, fmt.Sprintf("%s@%s", cfg.Repo, cfg.Version))
	}

	// download bigbang so we can peek inside
	chart := types.ZarfChart{
		Name:        _BB,
		Namespace:   _BB,
		URL:         repos[0],
		Version:     cfg.Version,
		ValuesFiles: cfg.ValuesFrom,
		GitPath:     "./chart",
	}

	helmCfg := helm.Helm{
		Chart: chart,
		Cfg: &types.PackagerConfig{
			State: types.ZarfState{},
		},
		BasePath: componentPath.Charts,
	}

	helmCfg.ChartLoadOverride = helmCfg.DownloadChartFromGit(_BB)

	// Template the chart so we can see what GitRepositories are being referenced in the
	// manifests created with the provided Helm
	template, err := helmCfg.TemplateChart()
	if err != nil {
		return component, fmt.Errorf("unable to template BigBang Chart: %w", err)
	}

	subPackageURLS := findURLs(template)
	repos[0] = fmt.Sprintf("%s@%s", repos[0], cfg.Version)
	repos = append(repos, subPackageURLS...)

	// Save the list of repos to be pulled down by Zarf
	component.Repos = repos

	// just select the images needed to support the repos this configuration of BigBang will need
	images, err := getImages(repos)
	if err != nil {
		return component, fmt.Errorf("unable to get bb images: %w", err)
	}

	// dedupe the list o fimages
	uniqueList := utils.Unique(images)

	// add the images to the component for Zarf to download
	component.Images = append(component.Images, uniqueList...)

	//Create the flux wrapper around BigBang for deployment
	manifest, err := getBigBangManifests(componentPath.Manifests, component.Extensions.BigBang)
	if err != nil {
		return component, err
	}

	component.Manifests = []types.ZarfManifest{manifest}
	return component, nil
}

// findURLs takes a list of yaml objects (as a string) and
// parses it for GitRepository objects that it then parses
// to return the list of git repos and tags needed.
func findURLs(t string) (urls []string) {
	// Break the template into separate resources.
	yamls, _ := utils.SplitYAMLToString([]byte(t))

	for _, y := range yamls {
		// Parse the resource into a shallow GitRepository object.
		var s fluxSrcCtrl.GitRepository
		if err := yaml.Unmarshal([]byte(y), &s); err != nil {
			continue
		}

		// If the resource is a GitRepository, parse it for the URL and tag.
		if s.Kind == "GitRepository" && s.Spec.URL != "" {
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

	return urls
}

// getImages identifies the list of images needed for the list of repos provided.
func getImages(repos []string) ([]string, error) {
	images := make([]string, 0)
	for _, r := range repos {
		is, err := helm.FindImagesForChartRepo(r, "chart")
		if err != nil {
			message.Warn(fmt.Sprintf("Could not pull images for chart %s: %s", r, err))
			continue
		}
		images = append(images, is...)
	}

	return images, nil
}

// getBigBangManifests creates the manifests component for deploying BigBang.
func getBigBangManifests(manifestDir string, bbCfg extensions.BigBang) (types.ZarfManifest, error) {
	//create a manifest component that we add to the zarf package for bigbang.
	manifest := types.ZarfManifest{
		Name:      _BB,
		Namespace: _BB,
	}

	// Helper function to marshal and write a manifest and add it to the component.
	addManifest := func(name string, data any) error {
		path := fmt.Sprintf("%s/%s", manifestDir, name)
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
	if err := addManifest("/gitrepository.yaml", manifestGitRepo(bbCfg)); err != nil {
		return manifest, err
	}

	// Create the zarf-credentials secret manifest.
	if err := addManifest("zarf-credentials.yaml", manifestZarfCredentials()); err != nil {
		return manifest, err
	}

	// Create the list of values manifests starting with zarf-credentials.
	hrValues := []fluxHelmCtrl.ValuesReference{{
		Kind: "Secret",
		Name: "zarf-credentials",
	}}

	for idx, path := range bbCfg.ValuesFrom {
		// Load the values file.
		file, err := os.ReadFile(path)
		if err != nil {
			return manifest, err
		}

		// Make a secret
		name := fmt.Sprintf("bigbang-values-%s", strconv.Itoa(idx))
		data := corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: _BB,
				Name:      name,
			},
			StringData: map[string]string{
				"values.yaml": string(file),
			},
		}

		if err := addManifest(name, data); err != nil {
			return manifest, err
		}

		// Add it to the list of valuesFrom for the HelmRelease
		hrValues = append(hrValues, fluxHelmCtrl.ValuesReference{
			Kind: "Secret",
			Name: name,
		})
	}

	if err := addManifest("helmrepository.yaml", manifestHelmRelease(hrValues)); err != nil {
		return manifest, err
	}

	return manifest, nil
}
