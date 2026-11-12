// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	t.Parallel()

	t.Run("uses the default retry count", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		err := Retry(context.Background(), 0, func() error {
			attempts++
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, DefaultRetries, attempts)
	})

	t.Run("rejects negative retries", func(t *testing.T) {
		t.Parallel()

		err := Retry(context.Background(), -1, func() error {
			return nil
		})
		require.EqualError(t, err, "retries cannot be negative")
	})

	t.Run("returns the operation error", func(t *testing.T) {
		t.Parallel()

		expected := errors.New("operation failed")
		err := Retry(context.Background(), 1, func() error {
			return expected
		})
		require.ErrorIs(t, err, expected)
	})
}

func TestRetryTransientFailureRecovery(t *testing.T) {
	t.Parallel()

	t.Run("succeeds after transient failures", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		err := Retry(context.Background(), 3, func() error {
			attempts++
			if attempts < 3 {
				return errors.New("transient network failure")
			}
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, 3, attempts)
	})

	t.Run("fails when all retries are exhausted", func(t *testing.T) {
		t.Parallel()

		attempts := 0
		permanentErr := errors.New("permanent failure")
		err := Retry(context.Background(), 3, func() error {
			attempts++
			return permanentErr
		})
		require.ErrorIs(t, err, permanentErr)
		require.Equal(t, 3, attempts)
	})

	t.Run("aborts on context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		err := Retry(ctx, 5, func() error {
			attempts++
			cancel()
			return errors.New("network error")
		})
		require.Error(t, err)
		require.Less(t, attempts, 5)
	})
}
