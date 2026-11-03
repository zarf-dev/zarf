// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import "fmt"

const (
	initAPIVersionV1Alpha1 = "v1alpha1"
	initAPIVersionV1Beta1  = "v1beta1"
	initPackageRepository  = "ghcr.io/zarf-dev/packages"
)

func getInitPackageURL(version, apiVersion string) (string, error) {
	switch apiVersion {
	case initAPIVersionV1Alpha1:
		return fmt.Sprintf("%s/init:%s", initPackageRepository, version), nil
	case initAPIVersionV1Beta1:
		return fmt.Sprintf("%s/%s/init:%s", initPackageRepository, apiVersion, version), nil
	default:
		return "", fmt.Errorf("unsupported init package API version %q: must be %s or %s", apiVersion, initAPIVersionV1Alpha1, initAPIVersionV1Beta1)
	}
}
