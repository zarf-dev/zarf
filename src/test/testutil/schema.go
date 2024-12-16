// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

type schemaFS struct {
	b []byte
}

func (m *schemaFS) ReadFile(_ string) ([]byte, error) {
	return m.b, nil
}

func (m *schemaFS) Open(_ string) (fs.File, error) {
	return nil, nil
}

// LoadSchema returns the schema file as a FS.
func LoadSchema(t *testing.T, path string) fs.ReadFileFS {
	t.Helper()

	_, testFilePath, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to determine the test file path")

	// Resolve the schema file's absolute path
	schemaPath := filepath.Join(filepath.Dir(testFilePath), path)

	b, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	return &schemaFS{b: b}
}
