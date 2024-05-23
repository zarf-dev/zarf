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

	pw.Write([]byte("foo"))

	require.Equal(t, "foo", buf.String())

	pw.Pause()

	pw.Write([]byte("bar"))

	require.Equal(t, "foo", buf.String())

	pw.Resume()

	pw.Write([]byte("baz"))

	require.Equal(t, "foobaz", buf.String())
}
