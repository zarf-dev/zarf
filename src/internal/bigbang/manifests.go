// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const bbV1ZarfCredentialsValues = `
registryCredentials:
  registry: "###ZARF_REGISTRY###"
  username: "zarf-pull"
  password: "###ZARF_REGISTRY_AUTH_PULL###"
git:
  existingSecret: "private-git-server"	# -- Chart created secrets with user defined values
  credentials:
  # -- HTTP git credentials, both username and password must be provided
    username: "###ZARF_GIT_PUSH###"
    password: "###ZARF_GIT_AUTH_PUSH###"
# -- Big Bang v1 Kyverno Support
kyvernopolicies:
  values:
    exclude:
      any:
      - resources:
          namespaces:
          - zarf # don't have Kyverno prevent Zarf from doing zarf things
          `

const bbV2ZarfCredentialsValues = `
registryCredentials:
  registry: "###ZARF_REGISTRY###"
  username: "zarf-pull"
  password: "###ZARF_REGISTRY_AUTH_PULL###"
git:
  existingSecret: "private-git-server"	# -- Chart created secrets with user defined values
  credentials:
  # -- HTTP git credentials, both username and password must be provided
    username: "###ZARF_GIT_PUSH###"
    password: "###ZARF_GIT_AUTH_PUSH###"
# -- Big Bang v2 Kyverno Support
kyvernoPolicies:
  values:
    exclude:
      any:
      - resources:
          namespaces:
          - zarf # don't have Kyverno prevent Zarf from doing zarf things
          `

func manifestZarfCredentials(version string) (corev1.Secret, error) {
	values := bbV1ZarfCredentialsValues

	semverVersion, err := semver.NewVersion(version)
	if err != nil {
		return corev1.Secret{}, err
	}
	if semverVersion.Major() == 2 {
		values = bbV2ZarfCredentialsValues
	}

	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: bb,
			Name:      "zarf-credentials",
		},
		StringData: map[string]string{
			"values.yaml": values,
		},
	}, nil
}
