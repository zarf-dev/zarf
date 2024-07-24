// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides global testing helper functions
package testutil

import (
	"context"
	"testing"
)

// TestContext takes a testing.T and returns a context that is
// attached to the test by t.Cleanup()
func TestContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
