package k8s

import (
	"errors"
	"regexp"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
)

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

// DetectDistro returns the matching distro or unknown if not found
func DetectDistro() (string, error) {
	message.Debugf("k8s.DetectDistro()")

	if config.DeployOptions.Distro != "" {
		return config.DeployOptions.Distro, nil
	}

	kindNodeRegex := regexp.MustCompile(`^kind://`)
	k3dNodeRegex := regexp.MustCompile(`^k3s://k3d-`)
	eksNodeRegex := regexp.MustCompile(`^aws:///`)
	gkeNodeRegex := regexp.MustCompile(`^gce://`)
	aksNodeRegex := regexp.MustCompile(`^azure:///subscriptions`)
	rke2Regex := regexp.MustCompile(`^rancher/rancher-agent:v2`)
	tkgRegex := regexp.MustCompile(`^projects\.registry\.vmware\.com/tkg/tanzu_core/`)

	nodes, err := GetNodes()
	message.Debug(nodes)
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

	namespaces, err := GetNamespaces()
	message.Debug(namespaces)
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

// GetArchitecture returns the cluster system architecture if found or an error if not
func GetArchitecture() (string, error) {
	message.Debugf("k8s.GetArchitecture()")
	nodes, err := GetNodes()

	message.Debug(nodes)

	if err != nil {
		return "", err
	}

	for _, node := range nodes.Items {
		return node.Status.NodeInfo.Architecture, nil
	}

	return "", errors.New("could not identify node architecture")
}
