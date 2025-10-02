// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injector

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
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
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
	c := &cluster.Cluster{
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

	err = StopInjection(ctx, c)
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

		err = StartInjection(ctx, tmpDir, images.PushConfig{
			Cluster: c,
		}, 31999, "test")
		require.NoError(t, err)

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

		cm, err := cs.CoreV1().ConfigMaps(state.ZarfNamespaceName).Get(ctx, "rust-binary", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, binData, cm.BinaryData["zarf-injector"])
		require.Equal(t, "test", cm.Labels["zarf.dev/package"])
	}

	err = StopInjection(ctx, c)
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
	pod := buildInjectionPod("injection-node", "docker.io/library/ubuntu:latest", resReq, "test")
	require.Equal(t, "injector", *pod.Name)
	require.Equal(t, "test", pod.Labels["zarf.dev/package"])
	b, err := json.MarshalIndent(pod, "", "  ")
	require.NoError(t, err)

	expected, err := os.ReadFile("./testdata/expected-injection-pod.json")
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(expected)), string(b))
}

func setupCluster(t *testing.T, nodes []corev1.Node, pods []corev1.Pod) *cluster.Cluster {
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
	return &cluster.Cluster{Clientset: cs}
}

func TestGetInjectorImageAndNode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cs := fake.NewClientset()

	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "no-resources",
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("400m"),
					corev1.ResourceMemory: resource.MustParse("50Mi"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "no-schedule-taint",
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "good",
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "no-execute-taint",
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Effect: corev1.TaintEffectNoExecute,
					},
				},
			},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
			},
		},
	}
	for i, node := range nodes {
		_, err := cs.CoreV1().Nodes().Create(ctx, &node, metav1.CreateOptions{})
		require.NoError(t, err)
		podName := fmt.Sprintf("pod-%d", i)
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				NodeName: node.Name,
				InitContainers: []corev1.Container{
					{
						Image: podName + "-init",
					},
				},
				Containers: []corev1.Container{
					{
						Image: podName + "-container",
					},
				},
				EphemeralContainers: []corev1.EphemeralContainer{
					{
						EphemeralContainerCommon: corev1.EphemeralContainerCommon{
							Image: podName + "-ephemeral",
						},
					},
				},
			},
		}
		_, err = cs.CoreV1().Pods(pod.Namespace).Create(ctx, &pod, metav1.CreateOptions{})
		require.NoError(t, err)
	}

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

		image, node, err := getInjectorImageAndNode(ctx, c, resReq)
		require.NoError(t, err)
		require.Equal(t, "nginx", image)
		require.Equal(t, "good", node)
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

		_, _, err := getInjectorImageAndNode(ctx, c, resReq)
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

		_, _, err := getInjectorImageAndNode(ctx, c, resReq)
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

		_, _, err := getInjectorImageAndNode(ctx, c, resReq)
		require.Error(t, err)
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

		_, _, err := getInjectorImageAndNode(ctx, c, resReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no suitable injector image or node")

		// But if we shrink the request to fit the remaining allocatable,
		// the injector should succeed
		smallReq := v1ac.ResourceRequirements().WithRequests(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"), // fits in 200m left
			corev1.ResourceMemory: resource.MustParse("50Mi"), // fits in 100Mi left
		})

		image, node, err := getInjectorImageAndNode(ctx, c, smallReq)
		require.NoError(t, err)
		require.Equal(t, "busybox", image)
		require.Equal(t, "crowded", node)
	})
}
