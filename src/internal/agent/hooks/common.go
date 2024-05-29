// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func addAgentLabel(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["zarf-agent"] = "patched"
	return labels
}

func getAnnotationPatch(currAnnotations map[string]string) operations.PatchOperation {
	return operations.ReplacePatchOperation("/metadata/annotations", addAgentLabel(currAnnotations))
}

func getLabelPatch(currLabels map[string]string) operations.PatchOperation {
	return operations.ReplacePatchOperation("/metadata/labels", addAgentLabel(currLabels))
}

// GetServiceInfoFromRegistryAddress gets the service info for a registry address if it is a NodePort
func GetServiceInfoFromRegistryAddress(ctx context.Context, stateRegistryAddress string) (string, error) {
	c, err := cluster.NewCluster()
	if err != nil {
		return "", fmt.Errorf("unable to get service information for the registry %q: %w", stateRegistryAddress, err)
	}

	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	// If this is an internal service then we need to look it up and
	_, _, clusterIP, port, err := cluster.ServiceInfoFromNodePortURL(serviceList.Items, stateRegistryAddress)
	if err != nil {
		message.Debugf("registry appears to not be a nodeport service, using original address %q", stateRegistryAddress)
		return stateRegistryAddress, nil
	}

	return fmt.Sprintf("%s:%d", clusterIP, port), nil
}
