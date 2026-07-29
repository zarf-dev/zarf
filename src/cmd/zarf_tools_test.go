// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

func TestRunWithRollback(t *testing.T) {
	t.Parallel()
	forwardErr := errors.New("forward failed")
	rollbackErr := errors.New("rollback failed")

	tests := []struct {
		name             string
		forward          error
		rollback         error
		wantRollbackCall bool
		wantErr          bool
		wantErrContains  string
	}{
		{
			name:             "forward succeeds, no rollback",
			forward:          nil,
			wantRollbackCall: false,
			wantErr:          false,
		},
		{
			name:             "forward fails, rollback succeeds",
			forward:          forwardErr,
			rollback:         nil,
			wantRollbackCall: true,
			wantErr:          true,
			wantErrContains:  "was rolled back to the previous credentials",
		},
		{
			name:             "forward fails, rollback fails",
			forward:          forwardErr,
			rollback:         rollbackErr,
			wantRollbackCall: true,
			wantErr:          true,
			wantErrContains:  "the cluster may be in an inconsistent state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rollbackCalled := false
			err := runWithRollback(context.Background(), "registry",
				func() error { return tt.forward },
				func() error { rollbackCalled = true; return tt.rollback },
			)
			require.Equal(t, tt.wantRollbackCall, rollbackCalled)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErrContains)
			// The original failure is always preserved in the returned error.
			require.ErrorIs(t, err, forwardErr)
			if tt.rollback != nil {
				require.ErrorIs(t, err, rollbackErr)
			}
		})
	}
}

// registryAuth reproduces the base64 auth string GenerateRegistryPullCreds writes for a pull user.
func registryAuth(user, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
}

// failFirstStateSave installs a reactor that fails only the first attempt to persist the Zarf state
// secret, letting later attempts (such as a rollback's save) succeed.
func failFirstStateSave(t *testing.T, c *cluster.Cluster) {
	t.Helper()
	fakeCS, ok := c.Clientset.(*fake.Clientset)
	require.True(t, ok)
	var stateSaves int
	fakeCS.PrependReactor("patch", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		pa, ok := action.(k8stesting.PatchAction)
		if ok && pa.GetName() == state.ZarfStateSecretName {
			stateSaves++
			if stateSaves == 1 {
				return true, nil, errors.New("injected state save failure")
			}
		}
		return false, nil, nil
	})
}

// seedCredsCluster returns a fake cluster with a namespace holding a Zarf-managed secret named
// managedSecretName and the Zarf state secret populated from s.
func seedCredsCluster(ctx context.Context, t *testing.T, s *state.State, managedSecretName string) *cluster.Cluster {
	t.Helper()
	c := &cluster.Cluster{Clientset: fake.NewClientset()}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	require.NoError(t, err)

	managedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedSecretName,
			Namespace: ns.Name,
			Labels:    map[string]string{state.ZarfManagedByLabel: "zarf"},
		},
	}
	_, err = c.Clientset.CoreV1().Secrets(ns.Name).Create(ctx, managedSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	b, err := json.Marshal(s)
	require.NoError(t, err)
	stateSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      state.ZarfStateSecretName,
			Namespace: state.ZarfNamespaceName,
		},
		Data: map[string][]byte{state.ZarfStateDataKey: b},
	}
	_, err = c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Create(ctx, stateSecret, metav1.CreateOptions{})
	require.NoError(t, err)

	return c
}

func externalRegistryState(pullPassword string) *state.State {
	return &state.State{
		RegistryInfo: state.RegistryInfo{
			RegistryMode: state.RegistryModeExternal,
			Address:      "registry.example.com",
			PullUsername: "pull-user",
			PullPassword: pullPassword,
			PushUsername: "push-user",
			PushPassword: "push-password",
		},
	}
}

func loadRegistryPullPassword(ctx context.Context, t *testing.T, c *cluster.Cluster) string {
	t.Helper()
	s, err := c.LoadState(ctx)
	require.NoError(t, err)
	return s.RegistryInfo.PullPassword
}

func TestUpdateRegistryCredsApplyState(t *testing.T) {
	t.Parallel()

	t.Run("external registry updates secrets and state", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		oldState := externalRegistryState("old-pull-password")
		newState := externalRegistryState("new-pull-password")
		c := seedCredsCluster(ctx, t, oldState, config.ZarfImagePullSecretName)

		o := &updateRegistryCredsOptions{confirm: true}
		require.NoError(t, o.applyState(ctx, c, newState))

		imageSecret, err := c.Clientset.CoreV1().Secrets("test").Get(ctx, config.ZarfImagePullSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Contains(t, string(imageSecret.Data[".dockerconfigjson"]), registryAuth("pull-user", "new-pull-password"))
		require.Equal(t, "new-pull-password", loadRegistryPullPassword(ctx, t, c))
	})

	t.Run("save failure rolls back to previous credentials", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		oldState := externalRegistryState("old-pull-password")
		newState := externalRegistryState("new-pull-password")
		c := seedCredsCluster(ctx, t, oldState, config.ZarfImagePullSecretName)

		// Fail only the first attempt to persist state so the forward pass fails after the image
		// pull secrets have already been rewritten, then let the rollback's save succeed.
		failFirstStateSave(t, c)

		o := &updateRegistryCredsOptions{confirm: true}
		err := runWithRollback(ctx, "registry",
			func() error { return o.applyState(ctx, c, newState) },
			func() error { return o.applyState(ctx, c, oldState) },
		)
		require.ErrorContains(t, err, "was rolled back to the previous credentials")

		imageSecret, err := c.Clientset.CoreV1().Secrets("test").Get(ctx, config.ZarfImagePullSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		dockerConfig := string(imageSecret.Data[".dockerconfigjson"])
		require.Contains(t, dockerConfig, registryAuth("pull-user", "old-pull-password"))
		require.NotContains(t, dockerConfig, registryAuth("pull-user", "new-pull-password"))
		require.Equal(t, "old-pull-password", loadRegistryPullPassword(ctx, t, c))
	})
}

func externalGitState(pullPassword string) *state.State {
	return &state.State{
		GitServer: state.GitServerInfo{
			Address:      "https://git.example.com",
			PullUsername: "pull-user",
			PullPassword: pullPassword,
			PushUsername: "push-user",
			PushPassword: "push-password",
		},
	}
}

func loadGitPullPassword(ctx context.Context, t *testing.T, c *cluster.Cluster) string {
	t.Helper()
	s, err := c.LoadState(ctx)
	require.NoError(t, err)
	return s.GitServer.PullPassword
}

func TestUpdateGitCredsApplyState(t *testing.T) {
	t.Parallel()

	t.Run("external git server updates secrets and state", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		oldState := externalGitState("old-pull-password")
		newState := externalGitState("new-pull-password")
		c := seedCredsCluster(ctx, t, oldState, config.ZarfGitServerSecretName)

		o := &updateGitCredsOptions{}
		require.NoError(t, o.applyState(ctx, c, oldState, newState))

		gitSecret, err := c.Clientset.CoreV1().Secrets("test").Get(ctx, config.ZarfGitServerSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "new-pull-password", gitSecret.StringData["password"])
		require.Equal(t, "new-pull-password", loadGitPullPassword(ctx, t, c))
	})

	t.Run("save failure rolls back to previous credentials", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		oldState := externalGitState("old-pull-password")
		newState := externalGitState("new-pull-password")
		c := seedCredsCluster(ctx, t, oldState, config.ZarfGitServerSecretName)

		// Fail only the first attempt to persist state so the forward pass fails after the git pull
		// secrets have already been rewritten, then let the rollback's save succeed.
		failFirstStateSave(t, c)

		o := &updateGitCredsOptions{}
		err := runWithRollback(ctx, "git server",
			func() error { return o.applyState(ctx, c, oldState, newState) },
			func() error { return o.applyState(ctx, c, newState, oldState) },
		)
		require.ErrorContains(t, err, "was rolled back to the previous credentials")

		gitSecret, err := c.Clientset.CoreV1().Secrets("test").Get(ctx, config.ZarfGitServerSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "old-pull-password", gitSecret.StringData["password"])
		require.Equal(t, "old-pull-password", loadGitPullPassword(ctx, t, c))
	})
}
