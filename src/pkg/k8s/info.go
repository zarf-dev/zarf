// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// GetArchitectures returns the cluster system architectures if found.
func (k *K8s) GetArchitectures(ctx context.Context) ([]string, error) {
	nodes, err := k.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	if len(nodes.Items) == 0 {
		return nil, errors.New("could not identify node architecture")
	}

	archMap := map[string]bool{}

	for _, node := range nodes.Items {
		archMap[node.Status.NodeInfo.Architecture] = true
	}

	architectures := []string{}

	for arch := range archMap {
		architectures = append(architectures, arch)
	}

	return architectures, nil
}

// GetServerVersion retrieves and returns the k8s revision.
func (k *K8s) GetServerVersion() (version string, err error) {
	versionInfo, err := k.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("unable to get Kubernetes version from the cluster : %w", err)
	}

	return versionInfo.String(), nil
}

// MakeLabels is a helper to format a map of label key and value pairs into a single string for use as a selector.
func MakeLabels(labels map[string]string) string {
	var out []string
	for key, value := range labels {
		out = append(out, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(out, ",")
}
