// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package split

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		fileSize             int
		chunkSize            int
		expectedFileSize     int64
		expectedLastFileSize int64
		expectedFileCount    int
		expectedSha256Sum    string
	}{
		{
			name:                 "split evenly",
			fileSize:             2048,
			chunkSize:            16,
			expectedFileSize:     16,
			expectedLastFileSize: 16,
			expectedFileCount:    128,
			expectedSha256Sum:    "93ecad679eff0df493aaf5d7d615211b0f1d7a919016efb15c98f0b8efb1ba43",
		},
		{
			name:                 "split with remainder",
			fileSize:             2048,
			chunkSize:            10,
			expectedFileSize:     10,
			expectedLastFileSize: 8,
			expectedFileCount:    205,
			expectedSha256Sum:    "fe8460f4d53d3578aa37191acf55b3db7bbcb706056f4b6b02a0c70f24b0d95a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			name := "random"
			p := filepath.Join(dir, name)
			f, err := os.Create(p)
			require.NoError(t, err)
			b := make([]byte, tt.fileSize)
			for i := range tt.fileSize {
				b[i] = byte(tt.chunkSize)
			}
			require.NoError(t, err)
			_, err = f.Write(b)
			require.NoError(t, err)
			err = f.Close()
			require.NoError(t, err)

			err = File(context.Background(), p, tt.chunkSize)
			require.NoError(t, err)

			_, err = os.Stat(p)
			require.ErrorIs(t, err, os.ErrNotExist)
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, tt.expectedFileCount+1)
			for i, entry := range entries[1:] {
				require.Equal(t, fmt.Sprintf("%s.part%03d", name, i+1), entry.Name())

				fi, err := entry.Info()
				require.NoError(t, err)
				if i == len(entries)-2 {
					require.Equal(t, tt.expectedLastFileSize, fi.Size())
				} else {
					require.Equal(t, tt.expectedFileSize, fi.Size())
				}
			}

			b, err = os.ReadFile(filepath.Join(dir, fmt.Sprintf("%s.part000", name)))
			require.NoError(t, err)
			var data ZarfSplitPackageData
			err = json.Unmarshal(b, &data)
			require.NoError(t, err)
			require.Equal(t, tt.expectedFileCount, data.Count)
			require.Equal(t, int64(tt.fileSize), data.Bytes)
			require.Equal(t, tt.expectedSha256Sum, data.Sha256Sum)
		})
	}
}

func TestSplitDeleteExistingFiles(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	inputFilename := filepath.Join(tempDir, "testfile.txt")
	data := make([]byte, 50)
	err := os.WriteFile(inputFilename, data, 0644)
	require.NoError(t, err)
	// Create many fake split files
	for i := range 15 {
		_, err := os.Create(fmt.Sprintf("%s.part%03d", inputFilename, i))
		require.NoError(t, err)
	}

	chunkSize := 20
	err = File(context.Background(), inputFilename, chunkSize)
	require.NoError(t, err)

	// Verify fake split files were deleted
	_, err = os.Stat(inputFilename)
	require.ErrorIs(t, err, os.ErrNotExist)
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	// Header file + 3 data files
	require.Len(t, entries, 4)
}
