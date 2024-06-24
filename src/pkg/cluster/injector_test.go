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
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateInjectorConfigMap(t *testing.T) {
	t.Parallel()

	binData := []byte("foobar")
	binPath := filepath.Join(t.TempDir(), "bin")
	err := os.WriteFile(binPath, binData, 0o644)
	require.NoError(t, err)

	cs := fake.NewSimpleClientset()
	c := &Cluster{
		Clientset: cs,
	}

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		err = c.createInjectorConfigMap(ctx, binPath)
		require.NoError(t, err)
		cm, err := cs.CoreV1().ConfigMaps(ZarfNamespaceName).Get(ctx, "rust-binary", metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, binData, cm.BinaryData["zarf-injector"])
	}
}

func TestCreateService(t *testing.T) {
	t.Parallel()

	cs := fake.NewSimpleClientset()
	c := &Cluster{
		Clientset: cs,
	}

	expected, err := os.ReadFile("./testdata/expected-injection-service.json")
	require.NoError(t, err)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, err := c.createService(ctx)
		require.NoError(t, err)
		svc, err := cs.CoreV1().Services(ZarfNamespaceName).Get(ctx, "zarf-injector", metav1.GetOptions{})
		require.NoError(t, err)
		b, err := json.Marshal(svc)
		require.NoError(t, err)
		require.Equal(t, strings.TrimSpace(string(expected)), string(b))
	}
}

func TestBuildInjectionPod(t *testing.T) {
	t.Parallel()

	c := &Cluster{}
	pod, err := c.buildInjectionPod("injection-node", "docker.io/library/ubuntu:latest", []string{"foo", "bar"}, "shasum")
	require.NoError(t, err)
	require.Contains(t, pod.Name, "injector-")
	// Replace the random UUID in the pod name with a fixed placeholder for consistent comparison.
	pod.ObjectMeta.Name = "injector-UUID"
	b, err := json.Marshal(pod)
	require.NoError(t, err)
	expected, err := os.ReadFile("./testdata/expected-injection-pod.json")
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(expected)), string(b))
}

func TestImagesAndNodesForInjection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cs := fake.NewSimpleClientset()

	c := &Cluster{
		Clientset: cs,
	}

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
				NodeName: node.ObjectMeta.Name,
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

	getCtx, getCancel := context.WithTimeout(ctx, 1*time.Second)
	defer getCancel()
	result, err := c.getImagesAndNodesForInjection(getCtx)
	require.NoError(t, err)
	expected := imageNodeMap{
		"pod-2-init":      []string{"good"},
		"pod-2-container": []string{"good"},
		"pod-2-ephemeral": []string{"good"},
	}
	require.Equal(t, expected, result)
}
