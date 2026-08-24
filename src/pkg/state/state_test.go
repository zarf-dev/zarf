// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state manages references to a logical zarf deployment in k8s.
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	"github.com/zarf-dev/zarf/src/pkg/pki"
)

func TestAgentIsConfigured(t *testing.T) {
	t.Parallel()

	require.False(t, (&State{}).AgentIsConfigured())
	require.False(t, (&State{AgentInfo: AgentInfo{TLS: pki.GeneratedPKI{CA: []byte("ca"), Key: []byte("key")}}}).AgentIsConfigured())
	require.True(t, (&State{AgentInfo: AgentInfo{TLS: pki.GeneratedPKI{Cert: []byte("cert")}}}).AgentIsConfigured())
}

func TestStateReconcile(t *testing.T) {
	t.Parallel()

	legacyTLS := pki.GeneratedPKI{Cert: []byte("legacy-cert")}
	s := State{
		AgentTLS:             legacyTLS,
		AgentTLSUserProvided: true,
		AgentMutationPolicy:  MutationPolicyLabeled,
		RegistryInfo: RegistryInfo{
			NodePort: 1234,
		},
	}

	s.Reconcile()

	require.Equal(t, legacyTLS, s.AgentInfo.TLS)
	require.True(t, s.AgentInfo.TLSUserProvided)
	require.Equal(t, MutationPolicyLabeled, s.AgentInfo.MutationPolicy)
	require.Equal(t, 1234, s.RegistryInfo.Port)
	require.Equal(t, 1234, s.RegistryInfo.NodePort)
}

func TestStateAgentInfoSerialization(t *testing.T) {
	t.Parallel()

	agentInfo := AgentInfo{
		TLS: pki.GeneratedPKI{
			CA:   []byte("ca"),
			Cert: []byte("cert"),
			Key:  []byte("key"),
		},
		TLSUserProvided: true,
		MutationPolicy:  MutationPolicyLabeled,
	}

	s := State{}
	s.SetAgentInfo(agentInfo)

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Contains(t, raw, "agentInfo")
	require.Contains(t, raw, "agentTLS")
	require.Contains(t, raw, "agentTLSUserProvided")
	require.Contains(t, raw, "agentMutationPolicy")

	var legacy struct {
		AgentTLS             pki.GeneratedPKI `json:"agentTLS"`
		AgentTLSUserProvided bool             `json:"agentTLSUserProvided"`
		AgentMutationPolicy  MutationPolicy   `json:"agentMutationPolicy"`
	}
	require.NoError(t, json.Unmarshal(data, &legacy))
	require.Equal(t, agentInfo.TLS, legacy.AgentTLS)
	require.True(t, legacy.AgentTLSUserProvided)
	require.Equal(t, agentInfo.MutationPolicy, legacy.AgentMutationPolicy)
}

func TestRegistryInfoKnownPlainHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		registry      RegistryInfo
		wantPlainHTTP bool
		wantKnown     bool
	}{
		{name: "Zarf-managed mTLS means HTTPS", registry: RegistryInfo{RegistryMode: RegistryModeProxy, MTLSStrategy: MTLSStrategyZarfManaged}, wantKnown: true},
		{name: "internal registry without mTLS means plain HTTP", registry: RegistryInfo{RegistryMode: RegistryModeNodePort}, wantPlainHTTP: true, wantKnown: true},
		{name: "external registry without mTLS must negotiate", registry: RegistryInfo{RegistryMode: RegistryModeExternal}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plainHTTP, known := tt.registry.KnownPlainHTTP()
			require.Equal(t, tt.wantKnown, known)
			if known {
				require.Equal(t, tt.wantPlainHTTP, plainHTTP)
			}
		})
	}
}

func TestRegistryInfoResolvePlainHTTP(t *testing.T) {
	t.Parallel()

	const nonLocal = "registry.example.com"
	ctx := context.Background()

	// A scheme known from state wins outright; host and forcePlainHTTP are ignored.
	t.Run("Zarf-managed mTLS resolves to HTTPS", func(t *testing.T) {
		t.Parallel()
		ri := RegistryInfo{RegistryMode: RegistryModeProxy, MTLSStrategy: MTLSStrategyZarfManaged}
		got, err := ri.ResolvePlainHTTP(ctx, "127.0.0.1:5000", true, ocischeme.ProbeOptions{})
		require.NoError(t, err)
		require.False(t, got)
	})
	t.Run("internal registry resolves to plain HTTP", func(t *testing.T) {
		t.Parallel()
		ri := RegistryInfo{RegistryMode: RegistryModeNodePort}
		got, err := ri.ResolvePlainHTTP(ctx, nonLocal, false, ocischeme.ProbeOptions{})
		require.NoError(t, err)
		require.True(t, got)
	})

	// Unknown scheme + non-local host never probes; it defaults to forcePlainHTTP.
	t.Run("external non-local host defaults to forcePlainHTTP", func(t *testing.T) {
		t.Parallel()
		ri := RegistryInfo{RegistryMode: RegistryModeExternal}
		off, err := ri.ResolvePlainHTTP(ctx, nonLocal, false, ocischeme.ProbeOptions{})
		require.NoError(t, err)
		require.False(t, off)
		on, err := ri.ResolvePlainHTTP(ctx, nonLocal, true, ocischeme.ProbeOptions{})
		require.NoError(t, err)
		require.True(t, on)
	})

	// forcePlainHTTP short-circuits the probe even for a localhost registry.
	t.Run("localhost with forcePlainHTTP skips probe", func(t *testing.T) {
		t.Parallel()
		ri := RegistryInfo{RegistryMode: RegistryModeExternal}
		got, err := ri.ResolvePlainHTTP(ctx, "127.0.0.1:1", true, ocischeme.ProbeOptions{})
		require.NoError(t, err)
		require.True(t, got)
	})

	// Unknown scheme + localhost host is probed; a plain-HTTP responder yields true.
	t.Run("localhost registry is probed and negotiates plain HTTP", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		probeCtx := ocischeme.WithNegotiator(context.Background(), ocischeme.New(ocischeme.Options{}))
		ri := RegistryInfo{RegistryMode: RegistryModeExternal}
		got, err := ri.ResolvePlainHTTP(probeCtx, srv.Listener.Addr().String(), false, ocischeme.ProbeOptions{})
		require.NoError(t, err)
		require.True(t, got)
	})
}

// TODO: Change password gen method to make testing possible.
func TestMergeStateRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		initRegistry     RegistryInfo
		oldRegistry      RegistryInfo
		expectedRegistry RegistryInfo
	}{
		{
			name: "username is unmodified",
			oldRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldRegistry: RegistryInfo{
				Address:  fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort: 1,
				Port:     1,
			},
			expectedRegistry: RegistryInfo{
				Address:  fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort: 1,
				Port:     1,
			},
		},
		{
			name: "internal server explicit same password is preserved",
			oldRegistry: RegistryInfo{
				Address:      fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort:     1,
				Port:         1,
				PushPassword: "same-password",
				PullPassword: "same-password",
			},
			initRegistry: RegistryInfo{
				PushPassword: "same-password",
				PullPassword: "same-password",
			},
			expectedRegistry: RegistryInfo{
				Address:      fmt.Sprintf("%s:%d", helpers.IPV4Localhost, 1),
				NodePort:     1,
				Port:         1,
				PushPassword: "same-password",
				PullPassword: "same-password",
			},
		},
		{
			name: "init options merged",
			oldRegistry: RegistryInfo{
				PushUsername: "doesn't matter",
				PullUsername: "doesn't matter",
				Address:      "doesn't matter",
				NodePort:     0,
				Secret:       "doesn't matter",
			},
			initRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
				NodePort:     1,
				Port:         1,
				Secret:       "secret",
			},
			expectedRegistry: RegistryInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
				NodePort:     1,
				Port:         1,
				Secret:       "secret",
			},
		},
		{
			name: "init options not merged",
			expectedRegistry: RegistryInfo{
				PushUsername: "",
				PullUsername: "",
				Address:      "",
				NodePort:     0,
				Secret:       "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &State{
				RegistryInfo: tt.oldRegistry,
			}
			newState, err := Merge(oldState, MergeOptions{
				RegistryInfo: tt.initRegistry,
				Services:     NewServiceSet(RegistryKey),
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectedRegistry.PushUsername, newState.RegistryInfo.PushUsername)
			require.Equal(t, tt.expectedRegistry.PullUsername, newState.RegistryInfo.PullUsername)
			require.Equal(t, tt.expectedRegistry.Address, newState.RegistryInfo.Address)
			require.Equal(t, tt.expectedRegistry.NodePort, newState.RegistryInfo.NodePort)
			require.Equal(t, tt.expectedRegistry.Port, newState.RegistryInfo.Port)
			require.Equal(t, tt.expectedRegistry.Secret, newState.RegistryInfo.Secret)
			// Only check passwords if explicitly set in expected (non-empty means explicit expectation)
			if tt.expectedRegistry.PushPassword != "" {
				require.Equal(t, tt.expectedRegistry.PushPassword, newState.RegistryInfo.PushPassword)
			}
			if tt.expectedRegistry.PullPassword != "" {
				require.Equal(t, tt.expectedRegistry.PullPassword, newState.RegistryInfo.PullPassword)
			}
		})
	}
}

func TestMergeStateRegistryUsesTargetModeForPasswordGeneration(t *testing.T) {
	t.Parallel()

	t.Run("external to internal generates omitted passwords", func(t *testing.T) {
		oldState := &State{RegistryInfo: RegistryInfo{
			RegistryMode: RegistryModeExternal,
			PushPassword: "external-push-password",
			PullPassword: "external-pull-password",
		}}
		newState, err := Merge(oldState, MergeOptions{
			RegistryInfo: RegistryInfo{RegistryMode: RegistryModeNodePort},
			Services:     NewServiceSet(RegistryKey),
		})
		require.NoError(t, err)
		require.NotEqual(t, oldState.RegistryInfo.PushPassword, newState.RegistryInfo.PushPassword)
		require.NotEqual(t, oldState.RegistryInfo.PullPassword, newState.RegistryInfo.PullPassword)
	})
}

func TestMergeStateRegistryResolvedPort(t *testing.T) {
	t.Parallel()

	t.Run("preserves port when mode is omitted", func(t *testing.T) {
		oldState := &State{RegistryInfo: RegistryInfo{
			RegistryMode: RegistryModeNodePort,
			NodePort:     31999,
			Port:         31999,
			PushPassword: "push-password",
			PullPassword: "pull-password",
		}}
		newState, err := Merge(oldState, MergeOptions{
			RegistryInfo: RegistryInfo{
				PushUsername: "new-user",
				PushPassword: "push-password",
				PullPassword: "pull-password",
			},
			Services: NewServiceSet(RegistryKey),
		})
		require.NoError(t, err)
		require.Equal(t, 31999, newState.RegistryInfo.Port)
		require.Equal(t, 31999, newState.RegistryInfo.NodePort)
	})

	t.Run("clears port for resolved explicit mode", func(t *testing.T) {
		oldState := &State{RegistryInfo: RegistryInfo{
			RegistryMode: RegistryModeProxy,
			NodePort:     5000,
			Port:         5000,
			MTLSStrategy: MTLSStrategyZarfManaged,
		}}
		newState, err := Merge(oldState, MergeOptions{
			RegistryInfo: RegistryInfo{
				RegistryMode: RegistryModeExternal,
				MTLSStrategy: MTLSStrategyNone,
			},
			Services: NewServiceSet(RegistryKey),
		})
		require.NoError(t, err)
		require.Zero(t, newState.RegistryInfo.Port)
		require.Zero(t, newState.RegistryInfo.NodePort)
		require.Equal(t, RegistryModeExternal, newState.RegistryInfo.RegistryMode)
		require.Equal(t, MTLSStrategyNone, newState.RegistryInfo.MTLSStrategy)
	})
}

// TODO: Change password gen method to make testing possible.
func TestMergeStateGit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		initGitServer     GitServerInfo
		oldGitServer      GitServerInfo
		expectedGitServer GitServerInfo
	}{
		{
			name: "address and usernames are unmodified",
			oldGitServer: GitServerInfo{
				Address:      "address",
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
			expectedGitServer: GitServerInfo{
				Address:      "address",
				PushUsername: "push-user",
				PullUsername: "pull-user",
			},
		},
		{
			name: "internal server auto generate",
			oldGitServer: GitServerInfo{
				Address: ZarfInClusterGitServiceURL,
			},
			expectedGitServer: GitServerInfo{
				Address: ZarfInClusterGitServiceURL,
			},
		},
		{
			name: "internal server explicit same password is preserved",
			oldGitServer: GitServerInfo{
				Address:      ZarfInClusterGitServiceURL,
				PushPassword: "same-password",
				PullPassword: "same-password",
			},
			initGitServer: GitServerInfo{
				PushPassword: "same-password",
				PullPassword: "same-password",
			},
			expectedGitServer: GitServerInfo{
				Address:      ZarfInClusterGitServiceURL,
				PushPassword: "same-password",
				PullPassword: "same-password",
			},
		},
		{
			name: "init options merged",
			oldGitServer: GitServerInfo{
				Address:      "doesn't matter",
				PushUsername: "doesn't matter",
				PullUsername: "doesn't matter",
			},
			initGitServer: GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
			},
			expectedGitServer: GitServerInfo{
				PushUsername: "push-user",
				PullUsername: "pull-user",
				Address:      "address",
			},
		},
		{
			name: "empty init options not merged",
			expectedGitServer: GitServerInfo{
				PushUsername: "",
				PullUsername: "",
				Address:      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &State{
				GitServer: tt.oldGitServer,
			}
			newState, err := Merge(oldState, MergeOptions{
				GitServer: tt.initGitServer,
				Services:  NewServiceSet(GitKey),
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectedGitServer.PushUsername, newState.GitServer.PushUsername)
			require.Equal(t, tt.expectedGitServer.PullUsername, newState.GitServer.PullUsername)
			require.Equal(t, tt.expectedGitServer.Address, newState.GitServer.Address)
			// Only check passwords if explicitly set in expected (non-empty means explicit expectation)
			if tt.expectedGitServer.PushPassword != "" {
				require.Equal(t, tt.expectedGitServer.PushPassword, newState.GitServer.PushPassword)
			}
			if tt.expectedGitServer.PullPassword != "" {
				require.Equal(t, tt.expectedGitServer.PullPassword, newState.GitServer.PullPassword)
			}
		})
	}
}

func TestMergeStateArtifact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		initArtifactServer     ArtifactServerInfo
		oldArtifactServer      ArtifactServerInfo
		expectedArtifactServer ArtifactServerInfo
	}{
		{
			name: "username is unmodified",
			oldArtifactServer: ArtifactServerInfo{
				PushUsername: "push-user",
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushUsername: "push-user",
			},
		},
		{
			name: "old state is internal server auto generate push token",
			oldArtifactServer: ArtifactServerInfo{
				PushToken: "foobar",
				Address:   ZarfInClusterArtifactServiceURL,
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushToken: "",
				Address:   ZarfInClusterArtifactServiceURL,
			},
		},
		{
			name: "old state is not internal server auto generate push token but init options does not match",
			initArtifactServer: ArtifactServerInfo{
				PushToken: "hello world",
			},
			oldArtifactServer: ArtifactServerInfo{
				PushToken: "foobar",
				Address:   ZarfInClusterArtifactServiceURL,
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushToken: "hello world",
				Address:   ZarfInClusterArtifactServiceURL,
			},
		},
		{
			name: "init options merged",
			oldArtifactServer: ArtifactServerInfo{
				PushUsername: "doesn't matter",
				PushToken:    "doesn't matter",
				Address:      "doesn't matter",
			},
			initArtifactServer: ArtifactServerInfo{
				PushUsername: "user",
				PushToken:    "token",
				Address:      "address",
			},
			expectedArtifactServer: ArtifactServerInfo{
				PushUsername: "user",
				PushToken:    "token",
				Address:      "address",
			},
		},
		{
			name: "empty init options not merged",
			expectedArtifactServer: ArtifactServerInfo{
				PushUsername: "",
				PushToken:    "",
				Address:      "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			oldState := &State{
				ArtifactServer: tt.oldArtifactServer,
			}
			newState, err := Merge(oldState, MergeOptions{
				ArtifactServer: tt.initArtifactServer,
				Services:       NewServiceSet(ArtifactKey),
			})
			require.NoError(t, err)
			require.Equal(t, tt.expectedArtifactServer, newState.ArtifactServer)
		})
	}
}

func TestMergeStateAgent(t *testing.T) {
	t.Parallel()

	t.Run("auto-generate new certs", func(t *testing.T) {
		t.Parallel()
		agentTLS, err := pki.GeneratePKI("example.com")
		require.NoError(t, err)
		oldState := &State{
			AgentInfo: AgentInfo{TLS: agentTLS},
		}
		newState, err := Merge(oldState, MergeOptions{
			Services: NewServiceSet(AgentKey),
		})
		require.NoError(t, err)
		require.NotEqual(t, oldState.AgentInfo.TLS, newState.AgentInfo.TLS)
		require.False(t, newState.AgentInfo.TLSUserProvided)
	})

	t.Run("user-provided certs are used and provenance is set", func(t *testing.T) {
		t.Parallel()
		oldState := &State{}
		userTLS := pki.GeneratedPKI{
			CA:   []byte("user-ca"),
			Cert: []byte("user-cert"),
			Key:  []byte("user-key"),
		}
		newState, err := Merge(oldState, MergeOptions{
			Services: NewServiceSet(AgentKey),
			AgentTLS: &userTLS,
		})
		require.NoError(t, err)
		require.Equal(t, userTLS, newState.AgentInfo.TLS)
		require.True(t, newState.AgentInfo.TLSUserProvided)
	})

	t.Run("auto-generate resets user-provided provenance", func(t *testing.T) {
		t.Parallel()
		oldState := &State{
			AgentInfo: AgentInfo{
				TLS:             pki.GeneratedPKI{CA: []byte("old-ca")},
				TLSUserProvided: true,
			},
		}
		newState, err := Merge(oldState, MergeOptions{
			Services: NewServiceSet(AgentKey),
		})
		require.NoError(t, err)
		require.NotEqual(t, oldState.AgentInfo.TLS, newState.AgentInfo.TLS)
		require.False(t, newState.AgentInfo.TLSUserProvided)
	})
}

func TestMergeInstalledChartsForComponent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		existingCharts  []InstalledChart
		installedCharts []InstalledChart
		expectedCharts  []InstalledChart
	}{
		{
			name: "existing charts are merged",
			existingCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
				{
					Namespace: "default",
					ChartName: "chart2",
				},
			},
			installedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart3",
				},
			},
			expectedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
				{
					Namespace: "default",
					ChartName: "chart2",
				},
				{
					Namespace: "default",
					ChartName: "chart3",
				},
			},
		},
		{
			name: "overlapping charts are merged",
			existingCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
				{
					Namespace: "default",
					ChartName: "chart2",
				},
			},
			installedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
			},
			expectedCharts: []InstalledChart{
				{
					Namespace: "default",
					ChartName: "chart1",
				},
				{
					Namespace: "default",
					ChartName: "chart2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := MergeInstalledChartsForComponent(tt.existingCharts, tt.installedCharts, false)
			require.ElementsMatch(t, tt.expectedCharts, actual)
		})
	}
}

func TestRegistryCertSecretData(t *testing.T) {
	t.Parallel()

	certs := pki.GeneratedPKI{CA: []byte("ca"), Cert: []byte("cert"), Key: []byte("key")}

	t.Run("round trips", func(t *testing.T) {
		t.Parallel()
		got, err := RegistryCertFromSecretData(RegistryCertSecretData(certs))
		require.NoError(t, err)
		require.Equal(t, certs, got)
	})

	t.Run("writes the standard kubernetes.io/tls keys", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, map[string][]byte{
			"ca.crt":  []byte("ca"),
			"tls.crt": []byte("cert"),
			"tls.key": []byte("key"),
		}, RegistryCertSecretData(certs))
	})

	t.Run("rejects data missing any key", func(t *testing.T) {
		t.Parallel()
		for _, missing := range []string{RegistrySecretCAPath, RegistrySecretCertPath, RegistrySecretKeyPath} {
			data := RegistryCertSecretData(certs)
			delete(data, missing)

			_, err := RegistryCertFromSecretData(data)
			require.ErrorContains(t, err, "incomplete", "missing %s should be rejected", missing)
		}
	})

	t.Run("rejects absent data", func(t *testing.T) {
		t.Parallel()
		_, err := RegistryCertFromSecretData(nil)
		require.ErrorContains(t, err, "incomplete")
	})
}
