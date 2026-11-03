// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package testutil provides global testing helper functions
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
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

// WritePackage writes a definition as zarf.yaml and authored source files into a
// test-owned directory. It does not load, render, or assemble package resources.
func WritePackage(t *testing.T, definition any, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	contents, err := yaml.Marshal(definition)
	require.NoError(t, err)
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), contents, 0o600))
	return dir
}
