// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"os"

	"github.com/Masterminds/semver/v3"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxSrcCtrl "github.com/fluxcd/source-controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
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

// manifestGitRepo generates a GitRepository object for the Big Bang umbrella repo.
func manifestGitRepo(version, repo string) (fluxSrcCtrl.GitRepository, error) {
	apiVersion := "source.toolkit.fluxcd.io/v1beta2"

	// Set apiVersion to v1 on BB v2.7.0 or higher falling back to v1beta2 as needed
	semverVersion, err := semver.NewVersion(version)
	if err != nil {
		return fluxSrcCtrl.GitRepository{}, err
	}
	c, err := semver.NewConstraint(">= 2.7.0")
	if err != nil {
		return fluxSrcCtrl.GitRepository{}, err
	}
	updateFlux := c.Check(semverVersion)
	if updateFlux {
		apiVersion = "source.toolkit.fluxcd.io/v1"
	}

	return fluxSrcCtrl.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxSrcCtrl.GitRepositoryKind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bb,
			Namespace: bb,
		},
		Spec: fluxSrcCtrl.GitRepositorySpec{
			URL:      repo,
			Interval: tenMins,
			Reference: &fluxSrcCtrl.GitRepositoryRef{
				Tag: version,
			},
		},
	}, nil
}

// getValuesFilesResource generates a Secret object for the Big Bang umbrella repo.
func getValuesFilesResource(path string) (unstructured.Unstructured, error) {
	// Read the file from the path.
	file, err := os.ReadFile(path)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	var resource unstructured.Unstructured
	if err := yaml.Unmarshal(file, &resource); err != nil {
		return unstructured.Unstructured{}, err
	}

	return resource, nil
}

// manifestHelmRelease generates a HelmRelease object for the Big Bang umbrella repo.
func manifestHelmRelease(values []fluxHelmCtrl.ValuesReference) fluxHelmCtrl.HelmRelease {
	return fluxHelmCtrl.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxHelmCtrl.HelmReleaseKind,
			APIVersion: "helm.toolkit.fluxcd.io/v2beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bb,
			Namespace: bb,
		},
		Spec: fluxHelmCtrl.HelmReleaseSpec{
			Timeout: &tenMins,
			Chart: &fluxHelmCtrl.HelmChartTemplate{
				Spec: fluxHelmCtrl.HelmChartTemplateSpec{
					Chart: "./chart",
					SourceRef: fluxHelmCtrl.CrossNamespaceObjectReference{
						Kind: fluxSrcCtrl.GitRepositoryKind,
						Name: bb,
					},
				},
			},
			Install: &fluxHelmCtrl.Install{
				Remediation: &fluxHelmCtrl.InstallRemediation{
					Retries: -1,
				},
			},
			Upgrade: &fluxHelmCtrl.Upgrade{
				Remediation: &fluxHelmCtrl.UpgradeRemediation{
					Retries: 5,
				},
				CleanupOnFail: true,
			},
			Rollback: &fluxHelmCtrl.Rollback{
				CleanupOnFail: true,
			},
			ValuesFrom: values,
		},
	}
}
