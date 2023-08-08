// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import "fmt"

// GetServerVersion retrieves and return the k8s revision.
func (k *K8s) GetServerVersion() (version string, err error) {
	versionInfo, err := k.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("unable to get Kubernetes version from the cluster : %w", err)
	}

	return versionInfo.String(), nil
}
