// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"strings"
)

// List of supported distros via distro detection.
const (
	DistroIsUnknown       = "unknown"
	DistroIsK3s           = "k3s"
	DistroIsK3d           = "k3d"
	DistroIsKind          = "kind"
	DistroIsMicroK8s      = "microk8s"
	DistroIsEKS           = "eks"
	DistroIsEKSAnywhere   = "eksanywhere"
	DistroIsDockerDesktop = "dockerdesktop"
	DistroIsGKE           = "gke"
	DistroIsAKS           = "aks"
	DistroIsRKE2          = "rke2"
	DistroIsTKG           = "tkg"
)

// DetectDistro returns the matching distro or unknown if not found.
func (k *K8s) DetectDistro(ctx context.Context) (string, error) {
	kindNodeRegex := regexp.MustCompile(`^kind://`)
	k3dNodeRegex := regexp.MustCompile(`^k3s://k3d-`)
	eksNodeRegex := regexp.MustCompile(`^aws:///`)
	gkeNodeRegex := regexp.MustCompile(`^gce://`)
	aksNodeRegex := regexp.MustCompile(`^azure:///subscriptions`)
	rke2Regex := regexp.MustCompile(`^rancher/rancher-agent:v2`)
	tkgRegex := regexp.MustCompile(`^projects\.registry\.vmware\.com/tkg/tanzu_core/`)

	nodes, err := k.GetNodes(ctx)
	if err != nil {
		return DistroIsUnknown, errors.New("error getting cluster nodes")
	}

	// All nodes should be the same for what we are looking for
	node := nodes.Items[0]

	// Regex explanation: https://regex101.com/r/TIUQVe/1
	// https://github.com/rancher/k3d/blob/v5.2.2/cmd/node/nodeCreate.go#L187
	if k3dNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsK3d, nil
	}

	// Regex explanation: https://regex101.com/r/le7PRB/1
	// https://github.com/kubernetes-sigs/kind/pull/1805
	if kindNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsKind, nil
	}

	// https://github.com/kubernetes/cloud-provider-aws/blob/454ed784c33b974c873c7d762f9d30e7c4caf935/pkg/providers/v2/instances.go#L234
	if eksNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsEKS, nil
	}

	if gkeNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsGKE, nil
	}

	// https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/legacy-cloud-providers/azure/azure_wrap.go#L46
	if aksNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsAKS, nil
	}

	labels := node.GetLabels()
	for _, label := range labels {
		// kubectl get nodes --selector node.kubernetes.io/instance-type=k3s for K3s
		if label == "node.kubernetes.io/instance-type=k3s" {
			return DistroIsK3s, nil
		}
		// kubectl get nodes --selector microk8s.io/cluster=true for MicroK8s
		if label == "microk8s.io/cluster=true" {
			return DistroIsMicroK8s, nil
		}
	}

	if node.GetName() == "docker-desktop" {
		return DistroIsDockerDesktop, nil
	}

	for _, images := range node.Status.Images {
		for _, image := range images.Names {
			if rke2Regex.MatchString(image) {
				return DistroIsRKE2, nil
			}
			if tkgRegex.MatchString(image) {
				return DistroIsTKG, nil
			}
		}
	}

	namespaces, err := k.GetNamespaces(ctx)
	if err != nil {
		return DistroIsUnknown, errors.New("error getting namespace list")
	}

	// kubectl get ns eksa-system for EKS Anywhere
	for _, namespace := range namespaces.Items {
		if namespace.Name == "eksa-system" {
			return DistroIsEKSAnywhere, nil
		}
	}

	return DistroIsUnknown, nil
}

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
