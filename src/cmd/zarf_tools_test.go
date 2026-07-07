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
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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

func TestGetCredsArtifactDeprecationWarning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		args               []string
		artifactConfigured bool
		expectWarning      bool
	}{
		{
			name:               "no args with artifact server configured warns",
			artifactConfigured: true,
			expectWarning:      true,
		},
		{
			name:               "no args without artifact server does not warn",
			artifactConfigured: false,
			expectWarning:      false,
		},
		{
			name:               "artifact key warns",
			args:               []string{artifactKey},
			artifactConfigured: true,
			expectWarning:      true,
		},
		{
			name:               "registry key does not warn even when artifact configured",
			args:               []string{registryKey},
			artifactConfigured: true,
			expectWarning:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logBuf := new(bytes.Buffer)
			log, err := logger.New(logger.Config{
				Level:       logger.Warn,
				Format:      logger.FormatConsole,
				Destination: logBuf,
			})
			require.NoError(t, err)
			ctx := logger.WithContext(context.Background(), log)

			c := &cluster.Cluster{
				Clientset: fake.NewClientset(),
			}
			s := &state.State{Distro: "test"}
			if tt.artifactConfigured {
				s.ArtifactServer = state.ArtifactServerInfo{Address: "https://git-server.com"}
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

			getCredsOpts := getCredsOptions{
				outputFormat: outputTable,
				outputWriter: new(bytes.Buffer),
				cluster:      c,
			}
			err = getCredsOpts.run(ctx, tt.args)
			require.NoError(t, err)

			if tt.expectWarning {
				require.Contains(t, logBuf.String(), lang.ArtifactServerDeprecated)
			} else {
				require.NotContains(t, logBuf.String(), lang.ArtifactServerDeprecated)
			}
		})
	}
}
