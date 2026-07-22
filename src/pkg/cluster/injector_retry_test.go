// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/state"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

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

	err := client.ConfigMaps(state.ZarfNamespaceName).DeleteCollection(t.Context(), metav1.DeleteOptions{}, metav1.ListOptions{})

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

	err := client.ConfigMaps(state.ZarfNamespaceName).DeleteCollection(t.Context(), metav1.DeleteOptions{}, metav1.ListOptions{})

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

	err := client.ConfigMaps(state.ZarfNamespaceName).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(1), attempts.Load())
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

func TestStopInjectionDoesNotListLegacyPayloadConfigMaps(t *testing.T) {
	t.Parallel()

	cs := fake.NewClientset()
	c := &Cluster{Clientset: cs}

	err := c.StopInjection(t.Context())

	require.NoError(t, err)
	for _, action := range cs.Actions() {
		require.False(t, action.GetVerb() == "list" && action.GetResource().Resource == "configmaps")
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

func writeSuccessStatus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, &metav1.Status{Status: metav1.StatusSuccess})
}

func writeJSON(w http.ResponseWriter, object any) {
	if err := json.NewEncoder(w).Encode(object); err != nil {
		panic(err)
	}
}
