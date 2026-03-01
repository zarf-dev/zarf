// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

//go:build unix

package split

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitFile_ClosesChunkFDsPerIteration(t *testing.T) {
	// Verifies that SplitFile closes each chunk's file descriptor per loop
	// iteration, requiring only a constant number of FDs (source + one
	// destination at a time). We lower the process FD limit well below the
	// number of chunks to confirm no descriptors accumulate.
	//
	// This test does NOT call t.Parallel() because Setrlimit is process-wide
	// and would interfere with concurrently running tests.

	var original syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original))
	})

	// Allow enough FDs for the Go runtime and test infrastructure (~15),
	// plus the source file, but far fewer than the 50 chunks we create.
	// SplitFile closes each chunk file per iteration, so only ~2 extra FDs
	// are needed at any point.
	const fdLimit = 40
	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{
		Cur: fdLimit,
		Max: original.Max,
	})
	require.NoError(t, err)

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "bigfile")

	// 50 chunks of 64 bytes = 3200 bytes total — well above the FD limit.
	const chunkSize = 64
	const totalSize = chunkSize * 50
	require.NoError(t, os.WriteFile(srcPath, make([]byte, totalSize), 0644))

	_, err = SplitFile(t.Context(), srcPath, chunkSize)
	require.NoError(t, err)
}
