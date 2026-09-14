// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestLogBootstrapRuntimeDiagnostics(t *testing.T) {
	tests := []struct {
		name        string
		objects     []runtime.Object
		listError   bool
		wantWarning bool
	}{
		{
			name: "warns only for qualifying CRI-O while logging all runtimes",
			objects: []runtime.Object{
				testRuntimeNode("worker-crio", "cri-o://1.35.6-5.rhaos4.22.git41f610b.el9"),
				testRuntimeNode("worker-containerd", "containerd://2.2.0"),
				testRuntimeNode("worker-old", "cri-o://1.34.9"),
			},
			wantWarning: true,
		},
		{
			name: "logs inventory without warning for nonmatching runtimes",
			objects: []runtime.Object{
				testRuntimeNode("worker-containerd", "containerd://2.2.0"),
				testRuntimeNode("worker-malformed", "cri-o://not-a-version"),
			},
		},
		{
			name:      "logs lookup failures without warning",
			listError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			l, err := logger.New(logger.Config{
				Level:       logger.Debug,
				Format:      logger.FormatJSON,
				Destination: logger.Destination(&output),
			})
			require.NoError(t, err)

			cs := fake.NewClientset(tt.objects...)
			if tt.listError {
				cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("nodes forbidden")
				})
			}

			logBootstrapRuntimeDiagnostics(logger.WithContext(context.Background(), l), &cluster.Cluster{Clientset: cs}, "127.0.0.1:31999")

			logs := output.String()
			if tt.listError {
				require.Contains(t, logs, "unable to inspect node container runtimes")
				require.NotContains(t, logs, "CRI-O 1.35+ detected")
				return
			}
			require.Contains(t, logs, "detected node container runtimes")
			require.Contains(t, logs, "worker-containerd")
			require.Contains(t, logs, "containerd://2.2.0")
			if tt.wantWarning {
				require.Contains(t, logs, "CRI-O 1.35+ detected")
				require.Contains(t, logs, "worker-crio")
				require.Contains(t, logs, "127.0.0.1:31999")
				require.Contains(t, logs, "use an external registry")
			} else {
				require.Contains(t, logs, "worker-malformed")
				require.Contains(t, logs, "cri-o://not-a-version")
				require.NotContains(t, logs, "CRI-O 1.35+ detected")
			}
		})
	}
}

func testRuntimeNode(name, runtimeVersion string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{ContainerRuntimeVersion: runtimeVersion}},
	}
}
