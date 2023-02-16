// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing BigBang and Flux
package bigbang

import (
	"github.com/defenseunicorns/zarf/src/types/extensions"
	fluxHelmCtrl "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxSrcCtrl "github.com/fluxcd/source-controller/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func manifestZarfCredentials() corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: _BB,
			Name:      "zarf-credentials",
		},
		StringData: map[string]string{
			"values.yaml": `
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
kyvernopolicies:
  values:
    exclude:
      any:
      - resources:
          namespaces: 
          - zarf # don't have kyverno prevent zarf from doing zarf things
            `,
		},
	}
}

// manifestGitRepo generates a GitRepository object for the BigBang umbrella repo.
func manifestGitRepo(bbCfg extensions.BigBang) fluxSrcCtrl.GitRepository {
	return fluxSrcCtrl.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxSrcCtrl.GitRepositoryKind,
			APIVersion: "source.toolkit.fluxcd.io/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      _BB,
			Namespace: _BB,
		},
		Spec: fluxSrcCtrl.GitRepositorySpec{
			URL:      bbCfg.Repo,
			Interval: tenMins,
			Reference: &fluxSrcCtrl.GitRepositoryRef{
				Tag: bbCfg.Version,
			},
		},
	}
}

// manifestHelmRelease generates a HelmRelease object for the BigBang umbrella repo.
func manifestHelmRelease(values []fluxHelmCtrl.ValuesReference) fluxHelmCtrl.HelmRelease {
	return fluxHelmCtrl.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxHelmCtrl.HelmReleaseKind,
			APIVersion: "helm.toolkit.fluxcd.io/v2beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      _BB,
			Namespace: _BB,
		},
		Spec: fluxHelmCtrl.HelmReleaseSpec{
			Timeout: &tenMins,
			Chart: fluxHelmCtrl.HelmChartTemplate{
				Spec: fluxHelmCtrl.HelmChartTemplateSpec{
					Chart: "./chart",
					SourceRef: fluxHelmCtrl.CrossNamespaceObjectReference{
						Kind: fluxSrcCtrl.GitRepositoryKind,
						Name: _BB,
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
