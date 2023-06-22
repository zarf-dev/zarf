// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"github.com/Masterminds/semver/v3"
	plutoversionsfile "github.com/fairwindsops/pluto/v5"
	"github.com/fairwindsops/pluto/v5/pkg/api"
	goyaml "github.com/goccy/go-yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const k8sDeprecationComponent = "k8s"

// HandleDeprecations takes in an unstructured object and the version of kubernetes in a cluster and returns a converted version of that object and whether it was modified (if applicable)
func (k *K8s) HandleDeprecations(rawData *unstructured.Unstructured, kubernetesVersion semver.Version) (*unstructured.Unstructured, bool, error) {
	deprecatedVersionContent := &api.VersionFile{}
	err := goyaml.Unmarshal(plutoversionsfile.Content(), deprecatedVersionContent)
	if err != nil {
		return rawData, false, err
	}

	for _, deprecation := range deprecatedVersionContent.DeprecatedVersions {
		if deprecation.Component == k8sDeprecationComponent && deprecation.Kind == rawData.GetKind() && deprecation.Name == rawData.GetAPIVersion() {
			removedVersion, err := semver.NewVersion(deprecation.RemovedIn)
			if err != nil {
				return rawData, false, err
			}

			if removedVersion.LessThan(&kubernetesVersion) {
				if deprecation.ReplacementAPI != "" {
					rawData.SetAPIVersion(deprecation.ReplacementAPI)
					return rawData, true, nil
				}

				return nil, true, nil
			}
		}
	}

	return rawData, false, nil
}
