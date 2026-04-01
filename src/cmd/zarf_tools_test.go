// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetCreds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		outputFormat outputFormat
		file         string
	}{
		{
			name:         "json get creds",
			outputFormat: outputJSON,
			file:         "expected.json",
		},
		{
			name:         "yaml get creds",
			outputFormat: outputYAML,
			file:         "expected.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			c := &cluster.Cluster{
				Clientset: fake.NewClientset(),
			}

			s := &state.State{
				GitServer: state.GitServerInfo{
					Address:      "https://git-server.com",
					PushUsername: "push-user",
					PushPassword: "push-password",
					PullPassword: "pull-password",
					PullUsername: "pull-user",
				},
				ArtifactServer: state.ArtifactServerInfo{
					Address:      "https://git-server.com",
					PushUsername: "push-user",
					PushToken:    "push-password",
				},
				RegistryInfo: state.RegistryInfo{
					PullUsername: "pull-user",
					PushUsername: "push-user",
					PullPassword: "pull-password",
					PushPassword: "push-password",
					Address:      "127.0.0.1:30001",
					NodePort:     30001,
				},
				Distro: "test",
			}

			b, err := json.Marshal(s)
			require.NoError(t, err)
			secret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      state.ZarfStateSecretName,
					Namespace: state.ZarfNamespaceName,
				},
				Data: map[string][]byte{
					state.ZarfStateDataKey: b,
				},
			}
			_, err = c.Clientset.CoreV1().Secrets("zarf").Create(ctx, &secret, metav1.CreateOptions{})
			require.NoError(t, err)
			buf := new(bytes.Buffer)
			getCredsOpts := getCredsOptions{
				outputFormat: tt.outputFormat,
				outputWriter: buf,
				cluster:      c,
			}
			err = getCredsOpts.run(ctx, nil)
			require.NoError(t, err)
			b, err = os.ReadFile(filepath.Join("testdata", "get-creds", tt.file))
			require.NoError(t, err)
			if tt.outputFormat == outputJSON {
				require.JSONEq(t, string(b), buf.String())
			}
			if tt.outputFormat == outputYAML {
				require.YAMLEq(t, string(b), buf.String())
			}
		})
	}
}

func TestGenKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		options    genKeyOptions
		keysExist  bool
		shouldFail bool
	}{
		{
			name:    "gen key",
			options: genKeyOptions{},
		},
		{
			name: "gen key password",
			options: genKeyOptions{
				password: "test-password",
			},
		},
		{
			name: "gen key password-stdin",
			options: genKeyOptions{
				passwordStdin: true,
			},
		},
		{
			name:       "gen key (key exists)",
			options:    genKeyOptions{},
			keysExist:  true,
			shouldFail: true,
		},
		{
			name: "gen key force (key exists)",
			options: genKeyOptions{
				force: true,
			},
			keysExist: true,
		},
		{
			name: "gen key interactive with password",
			options: genKeyOptions{
				interactive: true,
				password:    "test-password",
			},
			shouldFail: true,
		},
		{
			name: "gen key interactive with password-stdin",
			options: genKeyOptions{
				interactive:   true,
				passwordStdin: true,
			},
			shouldFail: true,
		},
		{
			name: "gen key password with password-stdin",
			options: genKeyOptions{
				password:      "test-password",
				passwordStdin: true,
			},
			shouldFail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()

			prvKeyName := filepath.Join(tempDir, "test.key")
			pubKeyName := filepath.Join(tempDir, "test.pub")

			if tt.keysExist {
				_, err := os.Create(prvKeyName)
				require.NoError(t, err, "Could not test existing keys or force because a file could not be created in the private key's place")
				_, err = os.Create(pubKeyName)
				require.NoError(t, err, "Could not test existing keys or force because a file could not be created in the public key's place")
			}

			err := tt.options.genKey(prvKeyName, pubKeyName)

			if tt.shouldFail {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			require.FileExists(t, prvKeyName, "private key did not generate")
			require.FileExists(t, pubKeyName, "public key did not generate")
		})
	}
}
