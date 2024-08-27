// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	krustytypes "sigs.k8s.io/kustomize/api/types"
)

// HelmReleaseDependency is a struct that represents a Flux Helm Release from an HR DependsOn list.
type HelmReleaseDependency struct {
	Metadata               metav1.ObjectMeta
	NamespacedDependencies []string
	NamespacedSource       string
	ValuesFrom             []fluxHelmCtrl.ValuesReference
}

// Name returns a namespaced name for the HelmRelease for dependency sorting.
func (h HelmReleaseDependency) Name() string {
	return getNamespacedNameFromMeta(h.Metadata)
}

// Dependencies returns a list of namespaced dependencies for the HelmRelease for dependency sorting.
func (h HelmReleaseDependency) Dependencies() []string {
	return h.NamespacedDependencies
}

// getFlux Creates a component to deploy Flux.
func getFlux(baseDir string, cfg *types.BigBang) (manifest types.ZarfManifest, images []string, err error) {
	localPath := path.Join(baseDir, "bb-ext-flux.yaml")
	kustomizePath := path.Join(baseDir, "kustomization.yaml")

	if cfg.Repo == "" {
		cfg.Repo = bbRepo
	}

	remotePath := fmt.Sprintf("%s//base/flux?ref=%s", cfg.Repo, cfg.Version)

	fluxKustomization := krustytypes.Kustomization{
		Resources: []string{remotePath},
	}

	for _, path := range cfg.FluxPatchFiles {
		absFluxPatchPath, _ := filepath.Abs(path)
		fluxKustomization.Patches = append(fluxKustomization.Patches, krustytypes.Patch{Path: absFluxPatchPath})
	}

	if err := utils.WriteYaml(kustomizePath, fluxKustomization, helpers.ReadWriteUser); err != nil {
		return manifest, images, fmt.Errorf("unable to write kustomization: %w", err)
	}

	// Perform Kustomization now to get the flux.yaml file.
	if err := kustomize.Build(baseDir, localPath, true); err != nil {
		return manifest, images, fmt.Errorf("unable to build kustomization: %w", err)
	}

	// Add the flux.yaml file to the component manifests.
	manifest = v1alpha1.ZarfManifest{
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

// readFluxImages finds the images Flux needs to deploy
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

// composeValues composes values from a Flux HelmRelease and Secrets Map
// (loosely based on upstream https://github.com/fluxcd/helm-controller/blob/main/controllers/helmrelease_controller.go#L551)
func composeValues(hr HelmReleaseDependency, secrets map[string]corev1.Secret, configMaps map[string]corev1.ConfigMap) (valuesMap chartutil.Values, err error) {
	valuesMap = chartutil.Values{}

	for _, v := range hr.ValuesFrom {
		var valuesData string
		namespacedName := getNamespacedNameFromStr(hr.Metadata.Namespace, v.Name)

		switch v.Kind {
		case "ConfigMap":
			cm, ok := configMaps[namespacedName]
			if !ok {
				return nil, fmt.Errorf("could not find values %s '%s'", v.Kind, namespacedName)
			}

			valuesData, ok = cm.Data[v.GetValuesKey()]
			if !ok {
				return nil, fmt.Errorf("missing key '%s' in %s '%s'", v.GetValuesKey(), v.Kind, namespacedName)
			}
		case "Secret":
			sec, ok := secrets[namespacedName]
			if !ok {
				return nil, fmt.Errorf("could not find values %s '%s'", v.Kind, namespacedName)
			}

			valuesData, ok = sec.StringData[v.GetValuesKey()]
			if !ok {
				return nil, fmt.Errorf("missing key '%s' in %s '%s'", v.GetValuesKey(), v.Kind, namespacedName)
			}
		default:
			return nil, fmt.Errorf("unsupported ValuesReference kind '%s'", v.Kind)
		}

		values, err := chartutil.ReadValues([]byte(valuesData))
		if err != nil {
			return nil, fmt.Errorf("unable to read values from key '%s' in %s '%s': %w", v.GetValuesKey(), v.Kind, hr.Name(), err)
		}

		valuesMap = helpers.MergeMapRecursive(valuesMap, values)
	}

	return valuesMap, nil
}

func getNamespacedNameFromMeta(o metav1.ObjectMeta) string {
	return getNamespacedNameFromStr(o.Namespace, o.Name)
}

func getNamespacedNameFromStr(namespace, name string) string {
	return fmt.Sprintf("%s.%s", namespace, name)
}
