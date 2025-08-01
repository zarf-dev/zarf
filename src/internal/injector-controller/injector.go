// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"
	"fmt"

	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// InjectionExecutor defines the interface for executing injection operations
type InjectionExecutor interface {
	// RunInjection executes the injection process
	Run(ctx context.Context, pod *corev1.Pod) error
	// RunWithOwner executes the injection process with an owner reference
	RunWithOwner(ctx context.Context, pod *corev1.Pod, owner *corev1.Pod) error
}

// clusterInjectionExecutor implements InjectionExecutor using cluster operations
type clusterInjectionExecutor struct {
	cluster *cluster.Cluster
}

// NewClusterInjectionExecutor creates a new InjectionExecutor using cluster operations
func NewClusterInjectionExecutor(cluster *cluster.Cluster) InjectionExecutor {
	return &clusterInjectionExecutor{
		cluster: cluster,
	}
}

// RunInjection executes the injection process
func (e *clusterInjectionExecutor) Run(ctx context.Context, proxyPod *corev1.Pod) error {
	return e.RunWithOwner(ctx, proxyPod, nil)
}

// RunWithOwner executes the injection process with an owner reference
func (e *clusterInjectionExecutor) RunWithOwner(ctx context.Context, proxyPod *corev1.Pod, owner *corev1.Pod) error {
	// FIXME: Get this info from state dynamically
	ipFamily := state.IPFamilyIPv4
	payloadCmNames := []string{}
	// FIXME: get shasum dynamically from cluster
	shasum := "fafcabcc56a3c76f5fce2767d86750c3082245081df7a625b1b428ce82a2fbaa"

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			"zarf-injector": "payload",
		},
	})
	if err != nil {
		return err
	}
	cmList, err := e.cluster.Clientset.CoreV1().ConfigMaps(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	for _, cm := range cmList.Items {
		payloadCmNames = append(payloadCmNames, cm.Name)
	}
	if err != nil {
		return err
	}

	nodeDetails, err := e.cluster.Clientset.CoreV1().Nodes().Get(ctx, proxyPod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	resReq := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(".5"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}).
		WithLimits(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		})

	if len(nodeDetails.Status.Images) == 0 {
		return fmt.Errorf("no images found on node: %s", nodeDetails.Name)
	}
	injectorImage := nodeDetails.Status.Images[0]
	for _, image := range nodeDetails.Status.Images[1:] {
		if image.SizeBytes < injectorImage.SizeBytes {
			injectorImage = image
		}
	}
	// This should never happen
	if len(injectorImage.Names) == 0 {
		return fmt.Errorf("node images has no names")
	}
	image := injectorImage.Names[0]
	var podSpec *v1ac.PodSpecApplyConfiguration
	if ipFamily == state.IPFamilyIPv6 {
		podSpec = cluster.BuildInjectionPodSpec(nodeDetails.Name, corev1.RestartPolicyAlways, image, payloadCmNames, shasum, resReq, v1ac.ContainerPort().WithContainerPort(5000)).
			WithHostNetwork(true)
	} else {
		podSpec = cluster.BuildInjectionPodSpec(nodeDetails.Name, corev1.RestartPolicyAlways, image, payloadCmNames,
			shasum, resReq, v1ac.ContainerPort().WithContainerPort(5000).WithHostIP("127.0.0.1").WithHostPort(5000))
	}
	podAc := v1ac.Pod("injector", state.ZarfNamespaceName).
		WithLabels(map[string]string{
			"app":               "zarf-injector",
			"zarf.dev/injector": "true",
			cluster.AgentLabel:  "ignore",
		}).
		WithSpec(podSpec)

	// Add owner reference if owner pod is provided
	if owner != nil {
		ownerRef := metav1ac.OwnerReference().
			WithAPIVersion("v1").
			WithKind("Pod").
			WithName(owner.Name).
			WithUID(owner.UID)
		podAc = podAc.WithOwnerReferences(ownerRef)
	}
	_, err = e.cluster.Clientset.CoreV1().Pods(*podAc.Namespace).Apply(ctx, podAc, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
	if err != nil {
		return fmt.Errorf("error creating pod in cluster: %w", err)
	}
	logger.From(ctx).Info("starting health checks ")
	podObj := []object.ObjMetadata{
		{
			GroupKind: schema.GroupKind{
				Group: proxyPod.GroupVersionKind().Group,
				Kind:  proxyPod.GroupVersionKind().Kind,
			},
			Namespace: proxyPod.Namespace,
			Name:      proxyPod.Name,
		},
	}
	err = healthchecks.WaitForReady(ctx, e.cluster.Watcher, podObj)
	if err != nil {
		return err
	}
	err = e.cluster.StopInjection(ctx, true)
	if err != nil {
		return err
	}
	return nil
}
