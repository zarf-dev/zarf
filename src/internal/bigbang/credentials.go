// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"github.com/Masterminds/semver/v3"
)

const bbV1ZarfCredentialsValues = `apiVersion: v1
kind: Secret
metadata:
  name: zarf-credentials
  namespace: bigbang
stringData:
  values.yaml: |
    registryCredentials:
      registry: "###ZARF_REGISTRY###"
      username: "zarf-pull"
      password: "###ZARF_REGISTRY_AUTH_PULL###"
    git:
      existingSecret: "private-git-server"	# -- Chart created secrets with user defined values
      credentials:
        username: "###ZARF_GIT_PUSH###" # -- HTTP git credentials, both username and password must be provided
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

const bbV2ZarfCredentialsValues = `apiVersion: v1
kind: Secret
metadata:
  name: zarf-credentials
  namespace: bigbang
stringData:
  values.yaml: |
    registryCredentials:
      registry: "###ZARF_REGISTRY###"
      username: "zarf-pull"
      password: "###ZARF_REGISTRY_AUTH_PULL###"
    git:
      existingSecret: "private-git-server"	# -- Chart created secrets with user defined values
      credentials:
        username: "###ZARF_GIT_PUSH###" # -- HTTP git credentials, both username and password must be provided
        password: "###ZARF_GIT_AUTH_PUSH###"
    kyvernoPolicies:
      values:
        exclude:
          any:
          - resources:
            namespaces:
            - zarf # don't have Kyverno prevent Zarf from doing zarf things
`

func manifestZarfCredentials(version string) (string, error) {
	semverVersion, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	if semverVersion.Major() == 2 {
		return bbV2ZarfCredentialsValues, nil
	}
	return bbV1ZarfCredentialsValues, nil
}
