// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package message

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPausableWriter(t *testing.T) {
	var buf bytes.Buffer

	pw := NewPausableWriter(&buf)

	n, err := pw.Write([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, 3, n)

	require.Equal(t, "foo", buf.String())

	pw.Pause()

	n, err = pw.Write([]byte("bar"))
	require.NoError(t, err)
	require.Equal(t, 3, n)

	require.Equal(t, "foo", buf.String())

	pw.Resume()

	n, err = pw.Write([]byte("baz"))
	require.NoError(t, err)
	require.Equal(t, 3, n)

	require.Equal(t, "foobaz", buf.String())
}
