// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsRetryableAdmissionWebhookError(t *testing.T) {
	webhookUnavailable := apierrors.NewInternalError(errors.New("failed calling webhook \"validate.example.test\": connection refused"))
	invalid := apierrors.NewInvalid(schema.GroupKind{Group: "example.test", Kind: "Widget"}, "widget", nil)

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "webhook transport failure",
			err:       webhookUnavailable,
			retryable: true,
		},
		{
			name:      "wrapped webhook transport failure",
			err:       fmt.Errorf("install failed: %w", webhookUnavailable),
			retryable: true,
		},
		{
			name:      "other internal error",
			err:       apierrors.NewInternalError(errors.New("etcd unavailable")),
			retryable: false,
		},
		{
			name:      "invalid resource",
			err:       invalid,
			retryable: false,
		},
		{
			name:      "forbidden resource",
			err:       apierrors.NewForbidden(schema.GroupResource{Group: "example.test", Resource: "widgets"}, "widget", errors.New("denied")),
			retryable: false,
		},
		{
			name:      "joined webhook and terminal error",
			err:       errors.Join(webhookUnavailable, invalid),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.retryable, isRetryableAdmissionWebhookError(tt.err))
			require.Equal(t, tt.retryable, isRetryableHelmChartError(tt.err))
		})
	}
}

func TestRetryHelmChartOperation(t *testing.T) {
	webhookUnavailable := apierrors.NewInternalError(errors.New("failed calling webhook \"validate.example.test\": connection refused"))
	invalid := apierrors.NewInvalid(schema.GroupKind{Group: "example.test", Kind: "Widget"}, "widget", nil)

	t.Run("retries webhook transport failures", func(t *testing.T) {
		var attempts atomic.Int32
		err := retryHelmChartOperation(context.Background(), "example-chart", 3, func() error {
			if attempts.Add(1) <= 2 {
				return webhookUnavailable
			}
			return nil
		})

		require.NoError(t, err)
		require.EqualValues(t, 3, attempts.Load())
	})

	t.Run("does not retry terminal failures", func(t *testing.T) {
		var attempts atomic.Int32
		err := retryHelmChartOperation(context.Background(), "example-chart", 3, func() error {
			attempts.Add(1)
			return invalid
		})

		require.Same(t, invalid, err)
		require.EqualValues(t, 1, attempts.Load())
	})
}
