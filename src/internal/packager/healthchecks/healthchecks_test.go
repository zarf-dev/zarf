// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package healthchecks run kstatus style health checks on a list of objects
package healthchecks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	dynamicfake "k8s.io/client-go/dynamic/fake"
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

func yamlToUnstructured(t *testing.T, yml string) *unstructured.Unstructured {
	t.Helper()
	m := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yml), &m)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: m}
}

func TestRunHealthChecks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		podYaml   string
		expectErr error
	}{
		{
			name:      "Pod is running",
			podYaml:   podCurrentYaml,
			expectErr: nil,
		},
		{
			name:      "Pod is never ready",
			podYaml:   podYaml,
			expectErr: context.DeadlineExceeded,
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
			pod := yamlToUnstructured(t, tt.podYaml)
			statusWatcher := watcher.NewDefaultStatusWatcher(fakeClient, fakeMapper)
			podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
			require.NoError(t, fakeClient.Tracker().Create(podGVR, pod, pod.GetNamespace()))
			objs := []v1alpha1.NamespacedObjectKindReference{
				{
					APIVersion: pod.GetAPIVersion(),
					Kind:       pod.GetKind(),
					Namespace:  pod.GetNamespace(),
					Name:       pod.GetName(),
				},
			}
			err := Run(ctx, statusWatcher, objs)
			if tt.expectErr != nil {
				require.ErrorIs(t, err, tt.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
