// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive contains the SDK for Zarf archival and compression.
package archive

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testDirPerm  = dirPerm
	testFilePerm = filePerm
)

// writeFile creates a file at path with given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), testFilePerm); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

// readFile returns the content of the file at path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	return string(data)
}

func TestCompressAndDecompress_MultipleFormats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	formats := []struct {
		name      string
		extension string
	}{
		{"tar", extensionTar},
		{"zip", extensionZip},
		{"tar.gz", extensionGz},
		{"tgz", extensionTgz},
		{"tar.bz2", extensionBz2},
		{"tbz2", extensionTbz2},
		{"tbz", extensionTbz},
		{"tar.xz", extensionXz},
		{"txz", extensionTxz},
		{"tar.zst", extensionZst},
		{"tzst", extensionTzst},
		{"tar.br", extensionBr},
		{"tbr", extensionTbr},
		{"tar.lz4", extensionLz4},
		{"tlz4", extensionTlz4},
		{"tar.lz", extensionLzip},
		{"tar.mz", extensionMz},
		{"tmz", extensionTmz},
	}

	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srcDir := t.TempDir()
			f1 := filepath.Join(srcDir, "file1.txt")
			f2 := filepath.Join(srcDir, "file2.txt")
			writeFile(t, f1, "hello world")
			writeFile(t, f2, "zarf testing")

			dest := filepath.Join(t.TempDir(), "archive"+tc.extension)
			require.NoError(t, Compress(ctx, []string{f1, f2}, dest, CompressOpts{}), "Compress failed for %s", tc.name)

			dstDir := t.TempDir()
			require.NoError(t, Decompress(ctx, dest, dstDir, DecompressOpts{}), "Decompress failed for %s", tc.name)

			got1 := readFile(t, filepath.Join(dstDir, "file1.txt"))
			require.Equal(t, "hello world", got1, "[%s] file1 content", tc.name)
			got2 := readFile(t, filepath.Join(dstDir, "file2.txt"))
			require.Equal(t, "zarf testing", got2, "[%s] file2 content", tc.name)
		})
	}
}

func TestCompressUnsupportedExtension(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := filepath.Join(t.TempDir(), "f.txt")
	writeFile(t, src, "data")
	dest := filepath.Join(t.TempDir(), "archive.foo")
	err := Compress(ctx, []string{src}, dest, CompressOpts{})
	if err == nil || !strings.Contains(err.Error(), "unsupported archive extension") {
		t.Errorf("expected unsupported extension error; got %v", err)
	}
}

func TestDecompressFiltered(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name        string
		setup       func(t *testing.T, ctx context.Context) (archivePath, outDir string, opts DecompressOpts)
		expectError string
		verify      func(t *testing.T, outDir string)
	}{
		{
			name: "Filtered_NotFound",
			setup: func(t *testing.T, ctx context.Context) (string, string, DecompressOpts) {
				srcDir := t.TempDir()
				file := filepath.Join(srcDir, "only.txt")
				writeFile(t, file, "uniquely present")
				destZip := filepath.Join(t.TempDir(), "only.zip")
				require.NoError(t, Compress(ctx, []string{file}, destZip, CompressOpts{}), "Compress failed")
				dstDir := t.TempDir()
				opts := DecompressOpts{Files: []string{"absent.txt"}, SkipValidation: false}
				return destZip, dstDir, opts
			},
			expectError: "absent.txt",
			verify:      nil,
		},
		{
			name: "Filtered_SkipValidation",
			setup: func(t *testing.T, ctx context.Context) (string, string, DecompressOpts) {
				srcDir := t.TempDir()
				file := filepath.Join(srcDir, "only.txt")
				writeFile(t, file, "content")
				destZip := filepath.Join(t.TempDir(), "only.zip")
				require.NoError(t, Compress(ctx, []string{file}, destZip, CompressOpts{}), "Compress failed")
				dstDir := t.TempDir()
				opts := DecompressOpts{Files: []string{"also_missing.txt"}, SkipValidation: true}
				return destZip, dstDir, opts
			},
			expectError: "",
			verify: func(t *testing.T, outDir string) {
				entries, err := os.ReadDir(outDir)
				require.NoError(t, err, "ReadDir failed")
				require.Empty(t, entries, "expected no files extracted")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			archivePath, outDir, opts := tc.setup(t, ctx)
			err := Decompress(ctx, archivePath, outDir, opts)
			if tc.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectError)
			} else {
				require.NoError(t, err)
				if tc.verify != nil {
					tc.verify(t, outDir)
				}
			}
		})
	}
}

func TestDecompressOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name   string
		setup  func(t *testing.T, ctx context.Context) (archivePath, outDir string, opts DecompressOpts)
		verify func(t *testing.T, outDir string)
	}{
		{
			name: "UnarchiveAll",
			setup: func(t *testing.T, ctx context.Context) (string, string, DecompressOpts) {
				tmp := t.TempDir()
				innerDir := filepath.Join(tmp, "inner")
				require.NoError(t, os.Mkdir(innerDir, testDirPerm))
				innerFile := filepath.Join(innerDir, "foo.txt")
				writeFile(t, innerFile, "nested content")
				innerTar := filepath.Join(tmp, "inner.tar")
				require.NoError(t, Compress(ctx, []string{innerFile}, innerTar, CompressOpts{}))
				outerDir := filepath.Join(tmp, "outer")
				require.NoError(t, os.Mkdir(outerDir, testDirPerm))
				outerTar := filepath.Join(tmp, "outer.tar")
				require.NoError(t, os.Rename(innerTar, filepath.Join(outerDir, "inner.tar")))
				require.NoError(t, Compress(ctx, []string{filepath.Join(outerDir, "inner.tar")}, outerTar, CompressOpts{}))
				outDir := filepath.Join(tmp, "out")
				opts := DecompressOpts{UnarchiveAll: true}
				return outerTar, outDir, opts
			},
			verify: func(t *testing.T, outDir string) {
				found := false
				err := filepath.Walk(outDir, func(path string, _ os.FileInfo, _ error) error {
					if filepath.Base(path) == "foo.txt" {
						found = true
						content := readFile(t, path)
						require.Equal(t, "nested content", content)
					}
					return nil
				})
				require.NoError(t, err, "Walk failed")
				require.True(t, found, "foo.txt not found after UnarchiveAll")
			},
		},
		{
			name: "OverwriteExisting",
			setup: func(t *testing.T, ctx context.Context) (string, string, DecompressOpts) {
				tmp := t.TempDir()
				origFile := filepath.Join(tmp, "orig.txt")
				writeFile(t, origFile, "original")
				archivePath := filepath.Join(tmp, "archive.tar.gz")
				require.NoError(t, Compress(ctx, []string{origFile}, archivePath, CompressOpts{}))
				outDir := filepath.Join(tmp, "out")
				require.NoError(t, Decompress(ctx, archivePath, outDir, DecompressOpts{}))
				outFile := filepath.Join(outDir, "orig.txt")
				require.Equal(t, "original", readFile(t, outFile))
				writeFile(t, origFile, "new content")
				archivePath2 := filepath.Join(tmp, "archive2.tar.gz")
				require.NoError(t, Compress(ctx, []string{origFile}, archivePath2, CompressOpts{}))
				opts := DecompressOpts{OverwriteExisting: true}
				return archivePath2, outDir, opts
			},
			verify: func(t *testing.T, outDir string) {
				outFile := filepath.Join(outDir, "orig.txt")
				require.Equal(t, "new content", readFile(t, outFile))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			archivePath, outDir, opts := tc.setup(t, ctx)
			require.NoError(t, Decompress(ctx, archivePath, outDir, opts))
			if tc.verify != nil {
				tc.verify(t, outDir)
			}
		})
	}
}
