package archive

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeFile creates a file at path with given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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
		{"zip", ".zip"},
		{"tar.gz", ".tar.gz"},
		{"tar.bz2", ".tar.bz2"},
		{"tar.xz", ".tar.xz"},
		{"tar.zst", ".tar.zst"},
		{"tar.lz4", ".tar.lz4"},
		{"tar.lz", ".tar.lz"},
		{"tar.mz", ".tar.mz"},
	}

	for _, tc := range formats {
		tc := tc // capture range variable
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
			if got1 != "hello world" {
				t.Errorf("[%s] file1 content = %q; want %q", tc.name, got1, "hello world")
			}
			got2 := readFile(t, filepath.Join(dstDir, "file2.txt"))
			if got2 != "zarf testing" {
				t.Errorf("[%s] file2 content = %q; want %q", tc.name, got2, "zarf testing")
			}
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

func TestDecompressFiltered_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srcDir := t.TempDir()
	file := filepath.Join(srcDir, "only.txt")
	writeFile(t, file, "uniquely present")
	destZip := filepath.Join(t.TempDir(), "only.zip")
	require.NoError(t, Compress(ctx, []string{file}, destZip, CompressOpts{}), "Compress failed")
	dstDir := t.TempDir()
	opts := DecompressOpts{Files: []string{"absent.txt"}, SkipValidation: false}
	err := Decompress(ctx, destZip, dstDir, opts)
	if err == nil || !strings.Contains(err.Error(), "absent.txt") {
		t.Errorf("expected error for missing file; got %v", err)
	}
}

func TestDecompressFiltered_SkipValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srcDir := t.TempDir()
	file := filepath.Join(srcDir, "only.txt")
	writeFile(t, file, "content")
	destZip := filepath.Join(t.TempDir(), "only.zip")
	require.NoError(t, Compress(ctx, []string{file}, destZip, CompressOpts{}), "Compress failed")
	dstDir := t.TempDir()
	opts := DecompressOpts{Files: []string{"also_missing.txt"}, SkipValidation: true}
	require.NoError(t, Decompress(ctx, destZip, dstDir, opts), "expected no error when skipValidation=true")
	entries, err := os.ReadDir(dstDir)
	require.NoError(t, err, "ReadDir failed")
	if len(entries) != 0 {
		t.Errorf("expected no files extracted; found %d entries", len(entries))
	}
}

func TestDecompress_UnarchiveAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmp := t.TempDir()

	// Create a nested tar archive inside another directory
	innerDir := filepath.Join(tmp, "inner")
	require.NoError(t, os.Mkdir(innerDir, 0o755))
	innerFile := filepath.Join(innerDir, "foo.txt")
	writeFile(t, innerFile, "nested content")

	innerTar := filepath.Join(tmp, "inner.tar")
	require.NoError(t, Compress(ctx, []string{innerFile}, innerTar, CompressOpts{}))

	// Now create an archive containing the inner tar
	outerDir := filepath.Join(tmp, "outer")
	require.NoError(t, os.Mkdir(outerDir, 0o755))
	outerTar := filepath.Join(tmp, "outer.tar")
	require.NoError(t, os.Rename(innerTar, filepath.Join(outerDir, "inner.tar")))
	require.NoError(t, Compress(ctx, []string{filepath.Join(outerDir, "inner.tar")}, outerTar, CompressOpts{}))

	// Decompress with UnarchiveAll
	outDir := filepath.Join(tmp, "out")
	opts := DecompressOpts{UnarchiveAll: true}
	require.NoError(t, Decompress(ctx, outerTar, outDir, opts))

	// Should have extracted foo.txt from the nested tar
	found := false
	_ = filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if filepath.Base(path) == "foo.txt" {
			found = true
			content := readFile(t, path)
			if content != "nested content" {
				t.Errorf("expected 'nested content', got %q", content)
			}
		}
		return nil
	})
	if !found {
		t.Error("foo.txt not found after UnarchiveAll")
	}
}

func TestDecompress_OverwriteExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmp := t.TempDir()

	// Create a file and archive it
	origFile := filepath.Join(tmp, "orig.txt")
	writeFile(t, origFile, "original")
	archivePath := filepath.Join(tmp, "archive.tar.gz")
	require.NoError(t, Compress(ctx, []string{origFile}, archivePath, CompressOpts{}))

	// Decompress once
	outDir := filepath.Join(tmp, "out")
	require.NoError(t, Decompress(ctx, archivePath, outDir, DecompressOpts{}))
	outFile := filepath.Join(outDir, "orig.txt")
	if got := readFile(t, outFile); got != "original" {
		t.Fatalf("expected original, got %q", got)
	}

	// Overwrite the file with new content and re-archive
	writeFile(t, origFile, "new content")
	archivePath2 := filepath.Join(tmp, "archive2.tar.gz")
	require.NoError(t, Compress(ctx, []string{origFile}, archivePath2, CompressOpts{}))

	// Decompress with OverwriteExisting
	opts := DecompressOpts{OverwriteExisting: true}
	require.NoError(t, Decompress(ctx, archivePath2, outDir, opts))
	if got := readFile(t, outFile); got != "new content" {
		t.Errorf("expected overwritten file to have 'new content', got %q", got)
	}
}
