// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package testutil provides global testing helper functions
package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// TestContext takes a testing.T and returns a context that is
// attached to the test by t.Cleanup()
func TestContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	l, err := logger.New(logger.ConfigDefault())
	require.NoError(t, err)
	ctx = logger.WithContext(ctx, l)
	return ctx
}
