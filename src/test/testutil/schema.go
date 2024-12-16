// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"io/fs"
	"os"
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

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return &schemaFS{b: b}
}
