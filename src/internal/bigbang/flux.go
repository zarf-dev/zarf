// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/pkg/helpers/v2"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"helm.sh/helm/v3/pkg/chartutil"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// HelmReleaseDependency is a struct that represents a Flux Helm Release from an HR DependsOn list.
type HelmReleaseDependency struct {
	Metadata               metav1.ObjectMeta
	NamespacedDependencies []string
	NamespacedSource       string
	ValuesFrom             []fluxHelmCtrl.ValuesReference
}

// readFluxImages finds the images Flux needs to deploy
func readFluxImages(fluxFilePath string) (images []string, err error) {
	contents, err := os.ReadFile(fluxFilePath)
	if err != nil {
		return images, fmt.Errorf("unable to read flux manifest: %w", err)
	}

	// Break the manifest into separate resources.
	yamls, err := utils.SplitYAML(contents)
	if err != nil {
		return nil, err
	}

	for _, yaml := range yamls {
		// Flux controllers are Deployments.
		if yaml.GetKind() == "Deployment" {
			deployment := v1.Deployment{}
			content := yaml.UnstructuredContent()

			// Convert the unstructured content into a Deployment.
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(content, &deployment); err != nil {
				return nil, err
			}

			pod := deployment.Spec.Template.Spec

			// Flux controllers do not have init containers today, but this is future proofing.
			for _, container := range pod.InitContainers {
				images = append(images, container.Image)
			}

			for _, container := range pod.Containers {
				images = append(images, container.Image)
			}
		}
	}

	return images, nil
}

// composeValues composes values from a Flux HelmRelease and Secrets Map
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
			return nil, fmt.Errorf("unable to read values from key '%s' in %s '%s': %w", v.GetValuesKey(), v.Kind, hr.Metadata.Name, err)
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
