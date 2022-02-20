package k8s

import (
	"fmt"
	"regexp"
)

const (
	DistroIsUnknown       = "unknown"
	DistroIsK3s           = "k3s"
	DistroIsK3d           = "k3d"
	DistroIsKind          = "kind"
	DistroIsMicroK8s      = "microk8s"
	DistroIsEks           = "eks"
	DistroIsEKSAnywhere   = "eksanywhere"
	DistroIsDockerDesktop = "dockerdesktop"

	// todo: more distros
)

func DetectDistro() (string, error) {
	kindNodeRegex := regexp.MustCompile(`^kind://`)
	k3dNodeRegex := regexp.MustCompile(`^k3s://k3d-`)
	eksNodeRegex := regexp.MustCompile(`aws:///`)

	nodes, err := GetNodes()
	if err != nil {
		return DistroIsUnknown, fmt.Errorf("error getting cluster nodes")
	}

	// Iterate over the nodes looking for label matches
	for _, node := range nodes.Items {
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

		// Regex explanation:
		// https://github.com/kubernetes/cloud-provider-aws/blob/454ed784c33b974c873c7d762f9d30e7c4caf935/pkg/providers/v2/instances.go#L234
		if eksNodeRegex.MatchString(node.Spec.ProviderID) {
			return DistroIsEks, nil
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

	}

	namespaces, err := GetNamespaces()
	if err != nil {
		return DistroIsUnknown, fmt.Errorf("error getting namesapce list")
	}

	// kubectl get ns eksa-system for EKS Anywhere
	for _, namespace := range namespaces.Items {
		if namespace.Name == "eksa-system" {
			return DistroIsEKSAnywhere, nil
		}
	}

	return DistroIsUnknown, nil
}

func GetArchitecture() (string, error) {
	nodes, err := GetNodes()
	if err != nil {
		return "", err
	}

	for _, node := range nodes.Items {
		return node.Status.NodeInfo.Architecture, nil
	}

	return "", fmt.Errorf("could not identify node architecture")
}
