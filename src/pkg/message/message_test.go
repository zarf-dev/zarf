// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package message

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pterm/pterm"
	"github.com/stretchr/testify/require"
)

func TestTableWritesToProvidedWriter(t *testing.T) {
	var defaultOutput bytes.Buffer
	pterm.SetDefaultOutput(&defaultOutput)
	t.Cleanup(func() { pterm.SetDefaultOutput(os.Stderr) })

	var output bytes.Buffer
	Table(&output, []string{"Name", "Status"}, [][]string{{"example", "ready"}})

	require.True(t, strings.HasPrefix(output.String(), "\n"), "the leading blank line must use the provided writer")
	for _, value := range []string{"Name", "Status", "example", "ready"} {
		require.Contains(t, output.String(), value)
	}
	require.Empty(t, defaultOutput.String(), "table output must not leak to PTerm's default writer")

	Table(io.Discard, []string{"Name"}, [][]string{{"discarded"}})
	require.Empty(t, defaultOutput.String(), "discarding a table must also discard its leading blank line")
}
