// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadFileByChunksRejectsInvalidSize(t *testing.T) {
	_, _, err := ReadFileByChunks("unused", 0)
	require.ErrorContains(t, err, "chunk size")
}
