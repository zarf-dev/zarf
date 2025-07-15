// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package injectorcontroller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckPodStatus(t *testing.T) {
	tests := []struct {
		name            string
		pod             *corev1.Pod
		expectInjection bool
	}{
		{
			name: "pod with ErrImagePull triggers injection",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: state.ZarfNamespaceName,
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-container",
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason:  "ErrImagePull",
									Message: "Failed to pull image",
								},
							},
						},
					},
				},
			},
			expectInjection: true,
		},
		{
			name: "pod with ImagePullBackOff triggers injection",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: state.ZarfNamespaceName,
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-container",
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason:  "ImagePullBackOff",
									Message: "Back-off pulling image",
								},
							},
						},
					},
				},
			},
			expectInjection: true,
		},
		{
			name: "pod with running container does not trigger injection",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: state.ZarfNamespaceName,
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-container",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.NewTime(time.Now()),
								},
							},
						},
					},
				},
			},
			expectInjection: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLogger, err := logger.New(logger.ConfigDefault())
			require.NoError(t, err)
			ctx := logger.WithContext(context.Background(), testLogger)
			client := fake.NewSimpleClientset()

			fakeInjector := NewFakeInjectionExecutor()
			cluster := &cluster.Cluster{
				Clientset: client,
			}
			controller := NewWithInjector(cluster, fakeInjector)

			controller.checkPodStatus(ctx, tt.pod, []string{"test-payload"})

			assert.Equal(t, tt.expectInjection, fakeInjector.RunInjectionCalled)
			assert.Equal(t, tt.expectInjection, fakeInjector.StopInjectionCalled)
		})
	}
}

func TestPollPods_Success(t *testing.T) {
	// Create test pods
	testPods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-1",
				Namespace: state.ZarfNamespaceName,
				Labels: map[string]string{
					"app": DaemonSetName,
				},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "test-container",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod-2",
				Namespace: state.ZarfNamespaceName,
				Labels: map[string]string{
					"app": DaemonSetName,
				},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "test-container",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ErrImagePull",
								Message: "Failed to pull image",
							},
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(&corev1.PodList{Items: testPods})

	// Create fake injector to track injection calls
	fakeInjector := NewFakeInjectionExecutor()

	cluster := &cluster.Cluster{
		Clientset: client,
	}
	controller := NewWithInjector(cluster, fakeInjector)

	testLogger, err := logger.New(logger.ConfigDefault())
	require.NoError(t, err)
	ctx := logger.WithContext(context.Background(), testLogger)

	err = controller.pollPods(ctx, []string{"test-payload"})
	require.NoError(t, err)

	// Verify that injection was triggered for the ErrImagePull pod
	assert.True(t, fakeInjector.RunInjectionCalled)
	assert.True(t, fakeInjector.StopInjectionCalled)
}

func TestPollPods_EmptyList(t *testing.T) {
	fakeInjector := NewFakeInjectionExecutor()
	client := fake.NewSimpleClientset()
	cluster := &cluster.Cluster{
		Clientset: client,
	}
	controller := NewWithInjector(cluster, fakeInjector)

	testLogger, err := logger.New(logger.ConfigDefault())
	require.NoError(t, err)
	ctx := logger.WithContext(context.Background(), testLogger)

	err = controller.pollPods(ctx, []string{})
	require.NoError(t, err)

	// Verify no injection calls were made
	assert.False(t, fakeInjector.RunInjectionCalled)
	assert.False(t, fakeInjector.StopInjectionCalled)
}
