// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"fmt"
	"os"
	"path"

	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/types/extensions"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// getFlux Creates a component to deploy Flux.
func getFlux(baseDir string, cfg *extensions.BigBang) (manifest types.ZarfManifest, images []string, err error) {
	localPath := path.Join(baseDir, "bb-ext-flux.yaml")
	remotePath := fmt.Sprintf("%s//base/flux?ref=%s", bbRepo, cfg.Version)

	// Perform Kustomzation now to get the flux.yaml file.
	if err := kustomize.BuildKustomization(remotePath, localPath, true); err != nil {
		return manifest, images, fmt.Errorf("unable to build kustomization: %w", err)
	}

	// Add the flux.yaml file to the component manifests.
	manifest = types.ZarfManifest{
		Name:      "flux-system",
		Namespace: "flux-system",
		Files:     []string{localPath},
	}

	// Read the flux.yaml file to get the images.
	if images, err = readFluxImages(localPath); err != nil {
		return manifest, images, fmt.Errorf("unable to read flux images: %w", err)
	}

	return manifest, images, nil
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
