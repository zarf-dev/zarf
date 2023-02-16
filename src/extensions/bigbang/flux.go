// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing BigBang and Flux
package bigbang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
)

// CreateFluxComponent Creates a component to deploy Flux.
func CreateFluxComponent(bbComponent types.ZarfComponent, bbCount int, gitCfg *git.Git) (fluxComponent types.ZarfComponent, err error) {
	fluxComponent.Name = "flux"
	fluxComponent.Required = bbComponent.Required

	fluxManifest := GetFluxManifest(bbComponent.Extensions.BigBang.Version)
	fluxComponent.Manifests = []types.ZarfManifest{fluxManifest}
	repo := _BB_REPO
	if bbComponent.Extensions.BigBang.Repo != "" {
		repo = bbComponent.Extensions.BigBang.Repo
	}
	images, err := findFluxImages(repo, bbComponent.Extensions.BigBang.Version, gitCfg)
	if err != nil {
		return fluxComponent, fmt.Errorf("unable to get flux images: %w", err)
	}

	fluxComponent.Images = images

	return fluxComponent, nil
}

// GetFluxManifest creates the manifests for deploying the specified version of BigBang via Kustomize.
func GetFluxManifest(version string) types.ZarfManifest {
	return types.ZarfManifest{
		Name:      "flux-system",
		Namespace: "flux-system",
		Kustomizations: []string{
			fmt.Sprintf("%s//base/flux?ref=%s", _BB_REPO, version),
		},
	}
}

// findFluxImages pulls the raw file from the https repo hosting bigbang.
// Will not work for private/offline/nongitlab based hostingsl
func findFluxImages(bigbangrepo, version string, gitCfg *git.Git) (images []string, err error) {
	spinner := message.NewProgressSpinner("Finding Flux Images")
	defer spinner.Stop()

	bigbangrepo = strings.TrimSuffix(bigbangrepo, ".git")

	path, err := gitCfg.DownloadRepoToTemp(bigbangrepo)
	if err != nil {
		spinner.Fatalf(err, "Error cloning bigbang repo")
		return images, err
	}
	defer os.RemoveAll(path)
	gitCfg.GitPath = path

	// Switch to the correct tag
	err = gitCfg.Checkout(version)
	if err != nil {
		spinner.Fatalf(err, "Unable to download provided git refrence: %v@%v", bigbangrepo, version)
	}

	fluxRawKustomization, err := os.ReadFile(filepath.Join(path, "base/flux/kustomization.yaml"))
	if err != nil {
		spinner.Fatalf(err, "Error reading kustomization object in flux directory")
		return images, err
	}
	fluxKustomization := kustypes.Kustomization{}
	err = yaml.Unmarshal([]byte(fluxRawKustomization), &fluxKustomization)
	if err != nil {
		spinner.Fatalf(err, "Error unmarshalling kustomization object in flux directory")
		return images, err
	}
	for _, i := range fluxKustomization.Images {
		images = append(images, fmt.Sprintf("%s:%s", i.NewName, i.NewTag))
	}
	return images, nil
}
