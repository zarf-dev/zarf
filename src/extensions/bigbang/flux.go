// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing BigBang and Flux
package bigbang

import (
	"fmt"
	"os"
	"path"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CreateFluxComponent Creates a component to deploy Flux.
func CreateFluxComponent(bbComponent types.ZarfComponent) (fluxComponent types.ZarfComponent, err error) {
	fluxComponent.Name = "flux"
	fluxComponent.Required = bbComponent.Required

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fluxComponent, fmt.Errorf("unable to create temp directory: %w", err)
	}

	localPath := path.Join(tmpDir, "flux.yaml")
	remotePath := fmt.Sprintf("%s//base/flux?ref=%s", _BB_REPO, bbComponent.Extensions.BigBang.Version)

	// Perform Kustomzation now to get the flux.yaml file.
	if err := kustomize.BuildKustomization(remotePath, localPath, true); err != nil {
		return fluxComponent, fmt.Errorf("unable to build kustomization: %w", err)
	}

	// Add the flux.yaml file to the component manifests.
	fluxComponent.Manifests = []types.ZarfManifest{{
		Name:      "flux-system",
		Namespace: "flux-system",
		Files:     []string{localPath},
	}}

	// Read the flux.yaml file to get the images.
	if fluxComponent.Images, err = readFluxImages(localPath); err != nil {
		return fluxComponent, fmt.Errorf("unable to read flux images: %w", err)
	}

	return fluxComponent, nil
}

func readFluxImages(localPath string) (images []string, err error) {
	contents, err := os.ReadFile(localPath)
	if err != nil {
		return images, fmt.Errorf("unable to read flux manifest: %w", err)
	}

	// Break the manifest into separate resources.
	yamls, _ := utils.SplitYAML(contents)

	// Loop through each resource and find the images.
	for _, yaml := range yamls {
		// Flux controllers are Deployments.
		if yaml.GetKind() == "Deployment" {
			deployment := v1.Deployment{}
			content := yaml.UnstructuredContent()

			// Convert the unstructured content into a Deployment.
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, &deployment); err != nil {
				return nil, fmt.Errorf("could not parse deployment: %w", err)
			}

			// Get the pod spec.
			pod := deployment.Spec.Template.Spec

			// Flux controllers do not have init containers today, but this is future proofing.
			for _, container := range pod.InitContainers {
				images = append(images, container.Image)
			}

			// Add the main containers.
			for _, container := range pod.Containers {
				images = append(images, container.Image)
			}

		}
	}

	return images, nil
}
