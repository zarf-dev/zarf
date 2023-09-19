// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackage_Files(t *testing.T) {
	pp := New("test")

	files := pp.Files()

	expected := map[string]string{
		"zarf.yaml":     "test/zarf.yaml",
		"checksums.txt": "test/checksums.txt",
	}

	require.Equal(t, expected, files)
}
