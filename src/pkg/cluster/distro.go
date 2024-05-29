// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"regexp"

	corev1 "k8s.io/api/core/v1"
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
func detectDistro(node corev1.Node, namespaces []corev1.Namespace) string {
	kindNodeRegex := regexp.MustCompile(`^kind://`)
	k3dNodeRegex := regexp.MustCompile(`^k3s://k3d-`)
	eksNodeRegex := regexp.MustCompile(`^aws:///`)
	gkeNodeRegex := regexp.MustCompile(`^gce://`)
	aksNodeRegex := regexp.MustCompile(`^azure:///subscriptions`)
	rke2Regex := regexp.MustCompile(`^rancher/rancher-agent:v2`)
	tkgRegex := regexp.MustCompile(`^projects\.registry\.vmware\.com/tkg/tanzu_core/`)

	// Regex explanation: https://regex101.com/r/TIUQVe/1
	// https://github.com/rancher/k3d/blob/v5.2.2/cmd/node/nodeCreate.go#L187
	if k3dNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsK3d
	}

	// Regex explanation: https://regex101.com/r/le7PRB/1
	// https://github.com/kubernetes-sigs/kind/pull/1805
	if kindNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsKind
	}

	// https://github.com/kubernetes/cloud-provider-aws/blob/454ed784c33b974c873c7d762f9d30e7c4caf935/pkg/providers/v2/instances.go#L234
	if eksNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsEKS
	}

	if gkeNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsGKE
	}

	// https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/legacy-cloud-providers/azure/azure_wrap.go#L46
	if aksNodeRegex.MatchString(node.Spec.ProviderID) {
		return DistroIsAKS
	}

	labels := node.GetLabels()
	for k, v := range labels {
		// kubectl get nodes --selector node.kubernetes.io/instance-type=k3s for K3s
		if k == "node.kubernetes.io/instance-type" && v == "k3s" {
			return DistroIsK3s
		}
		// kubectl get nodes --selector microk8s.io/cluster=true for MicroK8s
		if k == "microk8s.io/cluster" && v == "true" {
			return DistroIsMicroK8s
		}
	}

	if node.GetName() == "docker-desktop" {
		return DistroIsDockerDesktop
	}

	// TODO: Find a new detection method, by default the amount of images in the node status is limited.
	for _, images := range node.Status.Images {
		for _, image := range images.Names {
			if rke2Regex.MatchString(image) {
				return DistroIsRKE2
			}
			if tkgRegex.MatchString(image) {
				return DistroIsTKG
			}
		}
	}

	// kubectl get ns eksa-system for EKS Anywhere
	for _, namespace := range namespaces {
		if namespace.Name == "eksa-system" {
			return DistroIsEKSAnywhere
		}
	}

	return DistroIsUnknown
}
