// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestInjector(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset()
	c := &Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}
	cs.PrependReactor("delete-collection", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		delAction, ok := action.(k8stesting.DeleteCollectionActionImpl)
		if !ok {
			return false, nil, fmt.Errorf("action is not of type DeleteCollectionActionImpl")
		}
		if delAction.GetListRestrictions().Labels.String() != "zarf-injector=payload" {
			return false, nil, nil
		}
		gvr := delAction.Resource
		gvk := delAction.Resource.GroupVersion().WithKind("ConfigMap")
		list, err := cs.Tracker().List(gvr, gvk, delAction.Namespace)
		require.NoError(t, err)
		cmList, ok := list.(*corev1.ConfigMapList)
		require.True(t, ok)
		for _, cm := range cmList.Items {
			v, ok := cm.Labels["zarf-injector"]
			if !ok {
				continue
			}
			if v != "payload" {
				continue
			}
			err = cs.Tracker().Delete(gvr, delAction.Namespace, cm.Name)
			require.NoError(t, err)
		}
		return true, nil, nil
	})

	// Setup nodes and pods with images
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10"),
				corev1.ResourceMemory: resource.MustParse("100Gi"),
			},
		},
	}
	_, err := cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	require.NoError(t, err)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "good",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "node1",
			Containers: []corev1.Container{
				{
					Image: "ubuntu:latest",
				},
			},
		},
	}
	_, err = cs.CoreV1().Pods(pod.ObjectMeta.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.NoError(t, err)

	err = c.StopInjection(ctx)
	require.NoError(t, err)

	for range 2 {
		tmpDir := t.TempDir()
		binData := []byte("foobar")
		err := os.WriteFile(filepath.Join(tmpDir, "zarf-injector"), binData, 0o644)
		require.NoError(t, err)

		idx, err := random.Index(1, 1, 1)
		require.NoError(t, err)
		_, err = layout.Write(filepath.Join(tmpDir, "seed-images"), idx)
		require.NoError(t, err)

		selectedImage, _, err := c.StartInjection(ctx, tmpDir, t.TempDir(), nil, "test", "amd64", ZarfInjectorOptions{
			InjectorNodePort: 0,
			RegistryNodePort: 31999,
		})

		require.NoError(t, err)
		require.Equal(t, "ubuntu:latest", selectedImage)

		podList, err := cs.CoreV1().Pods(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, podList.Items, 1)
		require.Equal(t, "injector", podList.Items[0].Name)
		require.Equal(t, "test", podList.Items[0].Labels["zarf.dev/package"])

		svcList, err := cs.CoreV1().Services(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, svcList.Items, 1)
		require.Equal(t, "test", svcList.Items[0].Labels["zarf.dev/package"])
		expected, err := os.ReadFile("./testdata/expected-injection-service.json")
		require.NoError(t, err)
		svc, err := cs.CoreV1().Services(state.ZarfNamespaceName).Get(ctx, "zarf-injector", metav1.GetOptions{})
		// Managed fields are auto-set and contain timestamps
		svc.ManagedFields = nil
		require.NoError(t, err)
		b, err := json.MarshalIndent(svc, "", "  ")
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(string(expected)), string(b))

		cmList, err := cs.CoreV1().ConfigMaps(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		require.Len(t, cmList.Items, 2)
		cm, err := cs.CoreV1().ConfigMaps(state.ZarfNamespaceName).Get(ctx, "rust-binary", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, binData, cm.BinaryData["zarf-injector"])
		require.Equal(t, "test", cm.Labels["zarf.dev/package"])
	}

	err = c.StopInjection(ctx)
	require.NoError(t, err)

	podList, err := cs.CoreV1().Pods(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, podList.Items)
	svcList, err := cs.CoreV1().Services(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, svcList.Items)
	cmList, err := cs.CoreV1().ConfigMaps(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, cmList.Items)
}

func TestStartInjectionUsesRequestedImage(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	cs := fake.NewClientset()
	c := &Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "zarf-injector"), []byte("foobar"), 0o644))

	idx, err := random.Index(1, 1, 1)
	require.NoError(t, err)
	_, err = layout.Write(filepath.Join(tmpDir, "seed-images"), idx)
	require.NoError(t, err)

	image := "registry.example.com/zarf/injector:latest"
	selectedImage, _, err := c.StartInjection(ctx, tmpDir, t.TempDir(), nil, "test", "amd64", ZarfInjectorOptions{
		RegistryNodePort: 31999,
		Image:            image,
	})
	require.NoError(t, err)
	require.Equal(t, image, selectedImage)

	pod, err := cs.CoreV1().Pods(state.ZarfNamespaceName).Get(ctx, "injector", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, image, pod.Spec.Containers[0].Image)
	require.Empty(t, pod.Spec.NodeName)
}

func TestBuildInjectionPod(t *testing.T) {
	t.Parallel()

	resReq := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(".5"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}).
		WithLimits(
			corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			})
	// An unset IP family defaults to IPv4.
	pod := buildInjectionPod("injection-node", "docker.io/library/ubuntu:latest", []string{"foo", "bar"}, "shasum", resReq, "test", "", nil, nil)
	require.Equal(t, "injector", *pod.Name)
	require.Equal(t, "test", pod.Labels["zarf.dev/package"])
	require.Equal(t, []string{"/zarf-init/zarf-injector", "shasum", "0.0.0.0:5000"}, pod.Spec.Containers[0].Command)
	b, err := json.MarshalIndent(pod, "", "  ")
	require.NoError(t, err)

	expected, err := os.ReadFile("./testdata/expected-injection-pod.json")
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(expected)), string(b))

	ipv6Pod := buildInjectionPod("injection-node", "docker.io/library/ubuntu:latest", []string{"foo", "bar"}, "shasum", resReq, "test", state.IPFamilyIPv6, nil, nil)
	require.Equal(t, []string{"/zarf-init/zarf-injector", "shasum", "[::]:5000"}, ipv6Pod.Spec.Containers[0].Command)

	tolerationSeconds := int64(300)
	customPod := buildInjectionPod("injection-node", "docker.io/library/ubuntu:latest", []string{"foo"}, "shasum", resReq, "test", "", []corev1.Toleration{
		{
			Key:               "node-role.kubernetes.io/control-plane",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoSchedule,
			TolerationSeconds: &tolerationSeconds,
		},
	}, map[string]string{
		"kubernetes.io/arch": "amd64",
	})
	require.Equal(t, map[string]string{"kubernetes.io/arch": "amd64"}, customPod.Spec.NodeSelector)
	require.Len(t, customPod.Spec.Tolerations, 1)
	require.Equal(t, "node-role.kubernetes.io/control-plane", *customPod.Spec.Tolerations[0].Key)
	require.Equal(t, corev1.TolerationOperator(corev1.TolerationOpExists), *customPod.Spec.Tolerations[0].Operator)
	require.Equal(t, corev1.TaintEffect(corev1.TaintEffectNoSchedule), *customPod.Spec.Tolerations[0].Effect)
	require.Equal(t, int64(300), *customPod.Spec.Tolerations[0].TolerationSeconds)
}

func setupCluster(t *testing.T, nodes []corev1.Node, pods []corev1.Pod) *Cluster {
	t.Helper()
	cs := fake.NewClientset()
	ctx := context.Background()

	for _, node := range nodes {
		_, err := cs.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	for _, pod := range pods {
		_, err := cs.CoreV1().Pods(pod.Namespace).Create(ctx, &pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}
	return &Cluster{Clientset: cs}
}

func TestGetInjectorImageAndNode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Common resource requirement for injector
	resReq := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}).
		WithLimits(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		})

	t.Run("happy path", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "good"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{
					Architecture: "amd64",
				},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "good-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "good",
				Containers: []corev1.Container{{Image: "nginx"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		image, node, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "nginx", image)
		require.Equal(t, "good", node)
	})

	t.Run("wrong architecture", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "good"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{
					Architecture: "arm64",
				},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "good-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "good",
				Containers: []corev1.Container{{Image: "nginx"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		_, _, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no suitable injector image or node")
	})

	t.Run("insufficient resources", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "tiny"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
			},
		}}
		c := setupCluster(t, nodes, nil)

		_, _, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no suitable injector image or node")
	})

	t.Run("blocking taint", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "tainted"},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{{Effect: corev1.TaintEffectNoSchedule}},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "tainted-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "tainted",
				Containers: []corev1.Container{{Image: "nginx"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		_, _, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.Error(t, err)
	})

	t.Run("only zarf images", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "zarf-node"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "zarf-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "zarf-node",
				Containers: []corev1.Container{{Image: "127.0.0.1:5000/zarf"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		_, _, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.Error(t, err)
	})

	t.Run("prefers image from pod without imagePullSecrets", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "good"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
			},
		}}
		pods := []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-with-creds", Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName:         "good",
					Containers:       []corev1.Container{{Image: "private-image:latest"}},
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-without-creds", Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName:   "good",
					Containers: []corev1.Container{{Image: "public-image:latest"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
		}
		c := setupCluster(t, nodes, pods)

		image, node, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "public-image:latest", image)
		require.Equal(t, "good", node)
	})

	t.Run("does not use fallback when a later node has a no-creds image", func(t *testing.T) {
		nodes := []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-creds-only"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-public-image"},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			},
		}
		pods := []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-with-creds", Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName:         "node-creds-only",
					Containers:       []corev1.Container{{Image: "private-image:latest"}},
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-without-creds", Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName:   "node-public-image",
					Containers: []corev1.Container{{Image: "public-image:latest"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
		}
		c := setupCluster(t, nodes, pods)

		image, node, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "public-image:latest", image)
		require.Equal(t, "node-public-image", node)
	})

	t.Run("allocatable reduced by running pods", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "crowded"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
		}}

		// Create a pod that consumes most of the allocatable resources
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "heavy-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName: "crowded",
				Containers: []corev1.Container{{
					Image: "busybox",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("800m"),
							corev1.ResourceMemory: resource.MustParse("900Mi"),
						},
					},
				}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}

		c := setupCluster(t, nodes, pods)

		// Request more than the remaining resources (200m CPU / 100Mi mem left)
		resReq := v1ac.ResourceRequirements().WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("300m"),  // too big
			corev1.ResourceMemory: resource.MustParse("200Mi"), // too big
		})

		_, _, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no suitable injector image or node")

		// But if we shrink the request to fit the remaining allocatable,
		// the injector should succeed
		smallReq := v1ac.ResourceRequirements().WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"), // fits in 200m left
			corev1.ResourceMemory: resource.MustParse("50Mi"), // fits in 100Mi left
		})

		image, node, err := c.getInjectorImageAndNode(ctx, smallReq, "amd64", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "busybox", image)
		require.Equal(t, "crowded", node)
	})

	t.Run("tainted node tolerated with Exists operator", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane"},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				}},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "cp-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "control-plane",
				Containers: []corev1.Container{{Image: "coredns:1.10.0"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		tolerations := []corev1.Toleration{{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}}

		image, node, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", tolerations, nil)
		require.NoError(t, err)
		require.Equal(t, "coredns:1.10.0", image)
		require.Equal(t, "control-plane", node)
	})

	t.Run("tainted node tolerated with Equal operator", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{Name: "dedicated-node"},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{{
					Key:    "dedicated",
					Value:  "infra",
					Effect: corev1.TaintEffectNoSchedule,
				}},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "infra-pod", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "dedicated-node",
				Containers: []corev1.Container{{Image: "infra-agent:v1"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		tolerations := []corev1.Toleration{{
			Key:      "dedicated",
			Operator: corev1.TolerationOpEqual,
			Value:    "infra",
			Effect:   corev1.TaintEffectNoSchedule,
		}}

		image, node, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", tolerations, nil)
		require.NoError(t, err)
		require.Equal(t, "infra-agent:v1", image)
		require.Equal(t, "dedicated-node", node)
	})

	t.Run("nodeSelector filtering selects matching node", func(t *testing.T) {
		nodes := []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-1",
					Labels: map[string]string{
						"topology.kubernetes.io/zone": "zone-a",
						"node-type":                   "storage",
					},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-2",
					Labels: map[string]string{
						"topology.kubernetes.io/zone": "zone-b",
						"node-type":                   "compute",
					},
				},
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
					NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
				},
			},
		}
		pods := []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName:   "worker-1",
					Containers: []corev1.Container{{Image: "storage-image:v1"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "default"},
				Spec: corev1.PodSpec{
					NodeName:   "worker-2",
					Containers: []corev1.Container{{Image: "compute-image:v1"}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
		}
		c := setupCluster(t, nodes, pods)

		nodeSelector := map[string]string{
			"node-type": "compute",
		}

		image, node, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nodeSelector)
		require.NoError(t, err)
		require.Equal(t, "compute-image:v1", image)
		require.Equal(t, "worker-2", node)
	})

	t.Run("nodeSelector mismatch returns error", func(t *testing.T) {
		nodes := []corev1.Node{{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-1",
				Labels: map[string]string{
					"node-type": "storage",
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
			},
		}}
		pods := []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName:   "worker-1",
				Containers: []corev1.Container{{Image: "storage-image:v1"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}}
		c := setupCluster(t, nodes, pods)

		nodeSelector := map[string]string{
			"node-type": "gpu",
		}

		_, _, err := c.getInjectorImageAndNode(ctx, resReq, "amd64", nil, nodeSelector)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no suitable injector image or node")
	})
}

func TestLoadInjectionTemplates(t *testing.T) {
	t.Parallel()

	t.Run("valid multi-doc template", func(t *testing.T) {
		t.Parallel()
		manifest := `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    custom.io/sidecar: "false"
spec:
  containers:
  - name: injector
    resources:
      requests:
        cpu: "200m"
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    custom.io/lb: "internal"
`
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, injectionTemplateFileName), []byte(manifest), 0o600))
		pod, svc, err := loadInjectionTemplates(tmpDir)
		require.NoError(t, err)
		require.NotNil(t, pod)
		require.NotNil(t, svc)
		require.Equal(t, "false", pod.Annotations["custom.io/sidecar"])
		require.Equal(t, "internal", svc.Annotations["custom.io/lb"])
	})

	t.Run("file does not exist", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		pod, svc, err := loadInjectionTemplates(tmpDir)
		require.NoError(t, err)
		require.Nil(t, pod)
		require.Nil(t, svc)
	})

	t.Run("missing service in template", func(t *testing.T) {
		t.Parallel()
		manifest := `
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: injector
`
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, injectionTemplateFileName), []byte(manifest), 0o600))
		_, _, err := loadInjectionTemplates(tmpDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "one Pod and one Service")
	})

	t.Run("pod missing injector container", func(t *testing.T) {
		t.Parallel()
		manifest := `
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: sidecar
---
apiVersion: v1
kind: Service
`
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, injectionTemplateFileName), []byte(manifest), 0o600))
		_, _, err := loadInjectionTemplates(tmpDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must contain a container named \"injector\"")
	})

	t.Run("unsupported kind in template", func(t *testing.T) {
		t.Parallel()
		manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: foo
`
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, injectionTemplateFileName), []byte(manifest), 0o600))
		_, _, err := loadInjectionTemplates(tmpDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported kind \"Deployment\"")
	})
}

func TestMergeInjectionPodTemplate(t *testing.T) {
	t.Parallel()

	resReq := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse(".5"),
		})
	basePod := buildInjectionPod("node-1", "busybox:latest", []string{"cm-0"}, "shasum123", resReq, "pkg-test", state.IPFamilyIPv4, nil, nil)

	template := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "should-be-overridden",
			Annotations: map[string]string{
				"custom.annotation/test": "applied",
			},
			Labels: map[string]string{
				"custom-label": "custom-value",
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			Tolerations: []corev1.Toleration{{
				Key:      "dedicated",
				Operator: corev1.TolerationOpExists,
			}},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: func(i int64) *int64 { return &i }(2000),
			},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-registry-secret"}},
		},
	}

	merged, err := mergeInjectionPodTemplate(basePod, template)
	require.NoError(t, err)
	require.NotNil(t, merged)

	// Invariants preserved
	require.Equal(t, "injector", *merged.Name)
	require.Equal(t, state.ZarfNamespaceName, *merged.Namespace)
	require.Equal(t, "node-1", *merged.Spec.NodeName)
	require.Equal(t, corev1.RestartPolicyNever, *merged.Spec.RestartPolicy)

	// Invariant container command and working dir preserved
	require.Len(t, merged.Spec.Containers, 1)
	require.Equal(t, "injector", *merged.Spec.Containers[0].Name)
	require.Equal(t, "busybox:latest", *merged.Spec.Containers[0].Image)
	require.Equal(t, "/zarf-init", *merged.Spec.Containers[0].WorkingDir)
	require.Equal(t, []string{"/zarf-init/zarf-injector", "shasum123", "0.0.0.0:5000"}, merged.Spec.Containers[0].Command)

	// Invariant volume mounts and volumes preserved
	require.NotEmpty(t, merged.Spec.Containers[0].VolumeMounts)
	require.NotEmpty(t, merged.Spec.Volumes)

	// Merged user overrides
	require.Equal(t, "applied", merged.Annotations["custom.annotation/test"])
	require.Equal(t, "custom-value", merged.Labels["custom-label"])
	require.Equal(t, "pkg-test", merged.Labels[PackageLabel])
	require.Equal(t, "zarf-injector", merged.Labels["app"])
	require.Equal(t, "linux", merged.Spec.NodeSelector["kubernetes.io/os"])
	require.Len(t, merged.Spec.Tolerations, 1)
	require.Equal(t, "dedicated", *merged.Spec.Tolerations[0].Key)
	require.Equal(t, int64(2000), *merged.Spec.SecurityContext.RunAsUser)
	require.Len(t, merged.Spec.ImagePullSecrets, 1)
	require.Equal(t, "my-registry-secret", *merged.Spec.ImagePullSecrets[0].Name)
}

func TestMergeInjectionServiceTemplate(t *testing.T) {
	t.Parallel()

	baseSvc := v1ac.Service("zarf-injector", state.ZarfNamespaceName).
		WithSpec(v1ac.ServiceSpec().
			WithType(corev1.ServiceTypeNodePort).
			WithPorts(v1ac.ServicePort().WithPort(5000)).
			WithSelector(map[string]string{"app": "zarf-injector"})).
		WithLabels(map[string]string{PackageLabel: "pkg-test"})

	template := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "should-be-overridden",
			Annotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-internal": "true",
			},
			Labels: map[string]string{
				"custom-svc-label": "svc-value",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP, // Should be overridden back to NodePort
		},
	}

	opts := ZarfInjectorOptions{InjectorNodePort: 31999}
	merged, err := mergeInjectionServiceTemplate(baseSvc, template, opts, "pkg-test")
	require.NoError(t, err)
	require.NotNil(t, merged)

	// Invariants preserved
	require.Equal(t, "zarf-injector", *merged.Name)
	require.Equal(t, state.ZarfNamespaceName, *merged.Namespace)
	require.Equal(t, corev1.ServiceTypeNodePort, *merged.Spec.Type)
	require.Equal(t, map[string]string{"app": "zarf-injector"}, merged.Spec.Selector)
	require.Equal(t, int32(5000), *merged.Spec.Ports[0].Port)
	require.Equal(t, int32(31999), *merged.Spec.Ports[0].NodePort)

	// User overrides applied
	require.Equal(t, "true", merged.Annotations["service.beta.kubernetes.io/aws-load-balancer-internal"])
	require.Equal(t, "svc-value", merged.Labels["custom-svc-label"])
	require.Equal(t, "pkg-test", merged.Labels[PackageLabel])
}

func TestInjectionTemplateResources(t *testing.T) {
	t.Parallel()

	fallback := v1ac.ResourceRequirements().
		WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(".5"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}).
		WithLimits(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		})

	t.Run("nil template returns fallback", func(t *testing.T) {
		t.Parallel()
		res := injectionTemplateResources(nil, fallback)
		require.Equal(t, fallback, res)
	})

	t.Run("template without resources returns fallback", func(t *testing.T) {
		t.Parallel()
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "injector"}},
			},
		}
		res := injectionTemplateResources(pod, fallback)
		require.Equal(t, fallback, res)
	})

	t.Run("template with custom resources updates requirements", func(t *testing.T) {
		t.Parallel()
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "injector",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("250m"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						},
					},
				}},
			},
		}
		res := injectionTemplateResources(pod, fallback)
		require.Equal(t, resource.MustParse("250m"), (*res.Requests)[corev1.ResourceCPU])
		require.Equal(t, resource.MustParse("64Mi"), (*res.Requests)[corev1.ResourceMemory])
		require.Equal(t, resource.MustParse("2"), (*res.Limits)[corev1.ResourceCPU])
		require.Equal(t, resource.MustParse("256Mi"), (*res.Limits)[corev1.ResourceMemory])
	})
}

func TestStartInjectionWithTemplates(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset()
	c := &Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}

	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "test-node", Labels: map[string]string{"env": "test"}},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"},
		},
	}
	_, err := cs.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "zarf-injector"), []byte("dummy-bin"), 0o700))

	idx, err := random.Index(1, 1, 1)
	require.NoError(t, err)
	_, err = layout.Write(filepath.Join(tmpDir, "seed-images"), idx)
	require.NoError(t, err)

	podTemplate := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"zarf.dev/injector-test": "true"},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{"env": "test"},
			Containers:   []corev1.Container{{Name: "injector"}},
		},
	}
	svcTemplate := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"zarf.dev/service-test": "true"},
		},
	}

	opts := ZarfInjectorOptions{
		Image:            "public-image:v1",
		PodTemplate:      podTemplate,
		ServiceTemplate:  svcTemplate,
		InjectorNodePort: 32000,
	}

	img, port, err := c.StartInjection(ctx, tmpDir, t.TempDir(), nil, "test-pkg", "amd64", opts)
	require.NoError(t, err)
	require.Equal(t, "public-image:v1", img)
	require.Equal(t, 32000, port)

	pod, err := cs.CoreV1().Pods(state.ZarfNamespaceName).Get(ctx, "injector", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "true", pod.Annotations["zarf.dev/injector-test"])
	require.Equal(t, "test", pod.Spec.NodeSelector["env"])

	svc, err := cs.CoreV1().Services(state.ZarfNamespaceName).Get(ctx, "zarf-injector", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "true", svc.Annotations["zarf.dev/service-test"])
}

