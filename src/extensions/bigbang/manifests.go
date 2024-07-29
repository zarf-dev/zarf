// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxSrcCtrl "github.com/fluxcd/source-controller/api/v1"
	"github.com/zarf-dev/zarf/src/types/extensions"
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

func manifestZarfCredentials(version string) corev1.Secret {
	values := bbV1ZarfCredentialsValues

	semverVersion, err := semver.NewVersion(version)
	if err == nil && semverVersion.Major() == 2 {
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
	}
}

// manifestGitRepo generates a GitRepository object for the Big Bang umbrella repo.
func manifestGitRepo(cfg *extensions.BigBang) fluxSrcCtrl.GitRepository {
	apiVersion := "source.toolkit.fluxcd.io/v1beta2"

	// Set apiVersion to v1 on BB v2.7.0 or higher falling back to v1beta2 as needed
	semverVersion, _ := semver.NewVersion(cfg.Version)
	if semverVersion != nil {
		c, _ := semver.NewConstraint(">= 2.7.0")
		if c != nil {
			updateFlux, _ := c.Validate(semverVersion)
			if updateFlux && !cfg.SkipFlux {
				apiVersion = "source.toolkit.fluxcd.io/v1"
			}
		}
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
			URL:      cfg.Repo,
			Interval: tenMins,
			Reference: &fluxSrcCtrl.GitRepositoryRef{
				Tag: cfg.Version,
			},
		},
	}
}

// manifestValuesFile generates a Secret object for the Big Bang umbrella repo.
func manifestValuesFile(idx int, path string) (secret corev1.Secret, err error) {
	// Read the file from the path.
	file, err := os.ReadFile(path)
	if err != nil {
		return secret, err
	}

	// Get the base file name for this file.
	baseName := filepath.Base(path)

	// Define the name as the file name without the extension.
	baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Add the name prefix.
	name := fmt.Sprintf("bb-usr-vals-%d-%s", idx, baseName)

	// Create a secret with the file contents.
	secret = corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: bb,
			Name:      name,
		},
		StringData: map[string]string{
			"values.yaml": string(file),
		},
	}

	return secret, nil
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
