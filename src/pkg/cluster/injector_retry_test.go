// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/state"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestClientGoRetriesConfigMapApplyWithRetryAfter(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := newCoreV1Client(t, func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			writeTooManyRequests(w, "0")
			return
		}
		writeConfigMap(w)
	})

	_, err := client.ConfigMaps(state.ZarfNamespaceName).Apply(t.Context(), v1ac.ConfigMap("zarf-payload-000", state.ZarfNamespaceName), metav1.ApplyOptions{
		FieldManager: FieldManagerName,
		Force:        true,
	})

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestClientGoRetriesConfigMapDeleteWithRetryAfter(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := newCoreV1Client(t, func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			writeTooManyRequests(w, "0")
			return
		}
		writeSuccessStatus(w)
	})

	err := client.ConfigMaps(state.ZarfNamespaceName).Delete(t.Context(), "zarf-payload-000", metav1.DeleteOptions{})

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestClientGoDoesNotRetryHeaderlessTooManyRequests(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := newCoreV1Client(t, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		writeTooManyRequests(w, "")
	})

	_, err := client.ConfigMaps(state.ZarfNamespaceName).Apply(t.Context(), v1ac.ConfigMap("zarf-payload-000", state.ZarfNamespaceName), metav1.ApplyOptions{
		FieldManager: FieldManagerName,
		Force:        true,
	})

	require.Error(t, err)
	require.True(t, kerrors.IsTooManyRequests(err))
	require.Equal(t, int32(1), attempts.Load())
}

func TestClientGoBoundsRetriesWithRetryAfter(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := newCoreV1Client(t, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		writeTooManyRequests(w, "0")
	})

	_, err := client.ConfigMaps(state.ZarfNamespaceName).Apply(t.Context(), v1ac.ConfigMap("zarf-payload-000", state.ZarfNamespaceName), metav1.ApplyOptions{
		FieldManager: FieldManagerName,
		Force:        true,
	})

	require.Error(t, err)
	require.True(t, kerrors.IsTooManyRequests(err))
	require.Equal(t, int32(11), attempts.Load())
}

func TestClientGoStopsRetryingWhenContextExpires(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := newCoreV1Client(t, func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		writeTooManyRequests(w, "1")
	})
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()

	_, err := client.ConfigMaps(state.ZarfNamespaceName).Apply(ctx, v1ac.ConfigMap("zarf-payload-000", state.ZarfNamespaceName), metav1.ApplyOptions{
		FieldManager: FieldManagerName,
		Force:        true,
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(1), attempts.Load())
}

func TestStopInjectionRetriesTooManyRequests(t *testing.T) {
	t.Parallel()

	cs := fake.NewClientset()
	var attempts atomic.Int32
	cs.PrependReactor("delete", "pods", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		if attempts.Add(1) == 1 {
			return true, nil, kerrors.NewTooManyRequests("control plane throttled", 0)
		}
		return false, nil, nil
	})
	c := &Cluster{Clientset: cs}

	err := c.StopInjection(t.Context())

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestStopInjectionRetriesPayloadConfigMapCleanup(t *testing.T) {
	t.Parallel()

	cs := fake.NewClientset()
	var attempts atomic.Int32
	cs.PrependReactor("delete-collection", "configmaps", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		if attempts.Add(1) == 1 {
			return true, nil, kerrors.NewTooManyRequests("control plane throttled", 0)
		}
		return false, nil, nil
	})
	c := &Cluster{Clientset: cs}

	err := c.StopInjection(t.Context())

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestCreateInjectorConfigMapsRetriesTooManyRequests(t *testing.T) {
	t.Parallel()

	cs := fake.NewClientset()
	var attempts atomic.Int32
	cs.PrependReactor("patch", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patchAction, ok := action.(k8stesting.PatchAction)
		if !ok || patchAction.GetName() != "rust-binary" {
			return false, nil, nil
		}
		if attempts.Add(1) == 1 {
			return true, nil, kerrors.NewTooManyRequests("control plane throttled", 0)
		}
		return false, nil, nil
	})
	c := &Cluster{Clientset: cs}
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "zarf-injector"), []byte("injector"), 0o644))
	idx, err := random.Index(1, 1, 1)
	require.NoError(t, err)
	_, err = layout.Write(filepath.Join(tmpDir, "seed-images"), idx)
	require.NoError(t, err)

	_, _, err = c.CreateInjectorConfigMaps(t.Context(), tmpDir, t.TempDir(), nil, "test")

	require.NoError(t, err)
	require.Equal(t, int32(2), attempts.Load())
}

func TestCreatePayloadConfigMapsResumesAfterTooManyRequests(t *testing.T) {
	t.Parallel()

	cs := fake.NewClientset()
	attempts := map[string]int{}
	cs.PrependReactor("patch", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patchAction, ok := action.(k8stesting.PatchAction)
		if !ok {
			return false, nil, nil
		}
		name := patchAction.GetName()
		attempts[name]++
		if name == "zarf-payload-000" && attempts[name] == 1 {
			return true, nil, kerrors.NewTooManyRequests("control plane throttled", 0)
		}
		return false, nil, nil
	})
	c := &Cluster{Clientset: cs}
	tmpDir := t.TempDir()
	seedImagesDir := filepath.Join(tmpDir, "seed-images")
	idx, err := random.Index(1, 1, 1)
	require.NoError(t, err)
	_, err = layout.Write(seedImagesDir, idx)
	require.NoError(t, err)
	payload := make([]byte, 1024*768+1)
	_, err = cryptorand.Read(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(seedImagesDir, "payload"), payload, 0o644))

	cmNames, _, err := c.createPayloadConfigMaps(t.Context(), tmpDir, t.TempDir(), nil, "test")

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(cmNames), 2)
	require.Equal(t, 2, attempts[cmNames[0]])
	for _, cmName := range cmNames[1:] {
		require.Equal(t, 1, attempts[cmName])
	}
}

func TestRetryInjectorRequestBoundsHeaderlessTooManyRequests(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	retried, err := retryInjectorRequest(t.Context(), "test request", func() error {
		attempts.Add(1)
		return kerrors.NewTooManyRequests("control plane throttled", 0)
	})

	require.Error(t, err)
	require.True(t, kerrors.IsTooManyRequests(err))
	require.True(t, retried)
	require.Equal(t, int32(config.ZarfDefaultRetries), attempts.Load())
}

func TestRetryInjectorRequestDoesNotRetryServerDirectedThrottle(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	retried, err := retryInjectorRequest(t.Context(), "test request", func() error {
		attempts.Add(1)
		return kerrors.NewTooManyRequests("control plane throttled", 1)
	})

	require.Error(t, err)
	require.True(t, kerrors.IsTooManyRequests(err))
	require.False(t, retried)
	require.Equal(t, int32(1), attempts.Load())
}

func TestRetryInjectorRequestDoesNotRetryPermanentError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	retried, err := retryInjectorRequest(t.Context(), "test request", func() error {
		attempts.Add(1)
		return kerrors.NewBadRequest("invalid request")
	})

	require.Error(t, err)
	require.True(t, kerrors.IsBadRequest(err))
	require.False(t, retried)
	require.Equal(t, int32(1), attempts.Load())
}

func TestRetryInjectorRequestHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var attempts atomic.Int32
	retried, err := retryInjectorRequest(ctx, "test request", func() error {
		attempts.Add(1)
		return nil
	})

	require.ErrorIs(t, err, context.Canceled)
	require.False(t, retried)
	require.Zero(t, attempts.Load())
}

func newCoreV1Client(t *testing.T, handler http.HandlerFunc) corev1client.CoreV1Interface {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := corev1client.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	return client
}

func writeTooManyRequests(w http.ResponseWriter, retryAfter string) {
	if retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	writeJSON(w, kerrors.NewTooManyRequests("control plane throttled", 0).ErrStatus)
}

func writeConfigMap(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zarf-payload-000",
			Namespace: state.ZarfNamespaceName,
		},
	})
}

func writeSuccessStatus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, &metav1.Status{Status: metav1.StatusSuccess})
}

func writeJSON(w http.ResponseWriter, object any) {
	if err := json.NewEncoder(w).Encode(object); err != nil {
		panic(err)
	}
}
