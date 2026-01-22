// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package healthchecks run kstatus style health checks on a list of objects
package healthchecks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientfeatures "k8s.io/client-go/features"
	clientfeaturestesting "k8s.io/client-go/features/testing"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/testutil"
)

var podCurrentYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: good-pod
  namespace: ns
status:
  conditions:
  - type: Ready
    status: "True"
  phase: Running
`

var podYaml = `
apiVersion: v1
kind: Pod
metadata:
  name: in-progress-pod
  namespace: ns
`

var jobFailedYaml = `
apiVersion: batch/v1
kind: Job
metadata:
  name: failed-job
  namespace: ns
  generation: 1
status:
  failed: 1
  active: 0
  conditions:
  - type: Failed
    status: "True"
    reason: BackoffLimitExceeded
    message: "Job has reached the specified backoff limit"
`

func TestRunHealthChecks(t *testing.T) {
	// Workaround for Kubernetes client-go v0.35.0 breaking change where WatchListClient
	// feature gate (enabled by default in client-go v0.35.0) is incompatible
	// with fake client watch functionality. The fake client doesn't emit bookmark events
	// required by WatchListClient, causing watchers to hang indefinitely waiting for events.
	// References:
	// - KEP-3157: https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/3157-watch-list/README.md
	// - Issue #135895 (open, confirmed breaking change): https://github.com/kubernetes/kubernetes/issues/135895
	t.Parallel()
	clientfeaturestesting.SetFeatureDuringTest(t, clientfeatures.WatchListClient, false)

	tests := []struct {
		name       string
		podYamls   []string
		expectErrs []error
	}{
		{
			name:       "Pod is ready",
			podYamls:   []string{podCurrentYaml},
			expectErrs: nil,
		},
		{
			name:       "One pod is never ready",
			podYamls:   []string{podYaml, podCurrentYaml},
			expectErrs: []error{errors.New("in-progress-pod: Pod not ready, status is InProgress"), context.DeadlineExceeded},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			fakeMapper := testutil.NewFakeRESTMapper(
				v1.SchemeGroupVersion.WithKind("Pod"),
			)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)
			objs := []v1alpha1.NamespacedObjectKindReference{}
			for _, podYaml := range tt.podYamls {
				m := make(map[string]any)
				err := yaml.Unmarshal([]byte(podYaml), &m)
				require.NoError(t, err)
				pod := &unstructured.Unstructured{Object: m}
				podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
				err = fakeClient.Tracker().Create(podGVR, pod, pod.GetNamespace())
				require.NoError(t, err)
				objs = append(objs, v1alpha1.NamespacedObjectKindReference{
					APIVersion: pod.GetAPIVersion(),
					Kind:       pod.GetKind(),
					Namespace:  pod.GetNamespace(),
					Name:       pod.GetName(),
				})
			}

			err := Run(ctx, statusWatcher, objs)
			if tt.expectErrs != nil {
				require.EqualError(t, err, errors.Join(tt.expectErrs...).Error())
				return
			}
			require.NoError(t, err)
		})
	}

	t.Run("Failed is a terminal status", func(t *testing.T) {
		t.Parallel()
		fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
		fakeMapper := testutil.NewFakeRESTMapper(
			batchv1.SchemeGroupVersion.WithKind("Job"),
		)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)

		m := make(map[string]any)
		err := yaml.Unmarshal([]byte(jobFailedYaml), &m)
		require.NoError(t, err)
		job := &unstructured.Unstructured{Object: m}
		jobGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
		err = fakeClient.Tracker().Create(jobGVR, job, job.GetNamespace())
		require.NoError(t, err)

		objs := []v1alpha1.NamespacedObjectKindReference{
			{
				APIVersion: job.GetAPIVersion(),
				Kind:       job.GetKind(),
				Namespace:  job.GetNamespace(),
				Name:       job.GetName(),
			},
		}

		err = Run(ctx, statusWatcher, objs)

		require.Error(t, err)
		require.Contains(t, err.Error(), "failed-job: Job not ready, status is Failed")
		require.NotContains(t, err.Error(), "context deadline exceeded")
	})
}
