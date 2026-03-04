// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package archive

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeTarGz creates a tar.gz archive at dest from the given entries.
// Each entry is either a regular file or a symlink, controlled by the entry fields.
func makeTarGz(t *testing.T, dest string, entries []tarEntry) {
	t.Helper()
	f, err := os.Create(dest)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		switch {
		case e.linkTarget != "":
			typeflag := byte(tar.TypeSymlink)
			if e.isHardLink {
				typeflag = tar.TypeLink
			}
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name:     e.name,
				Typeflag: typeflag,
				Linkname: e.linkTarget,
			}))
		case e.isDir:
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name:     e.name,
				Typeflag: tar.TypeDir,
				Mode:     0o755,
			}))
		default:
			mode := int64(0o644)
			if e.mode != 0 {
				mode = e.mode
			}
			data := []byte(e.content)
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name:     e.name,
				Typeflag: tar.TypeReg,
				Mode:     mode,
				Size:     int64(len(data)),
			}))
			_, err := tw.Write(data)
			require.NoError(t, err)
		}
	}

	// Close in order: tar writer, gzip writer, file. Each must succeed
	// for the archive to be valid.
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())
}

type tarEntry struct {
	name       string
	content    string
	linkTarget string
	isDir      bool
	isHardLink bool  // if true and linkTarget is set, create TypeLink instead of TypeSymlink
	mode       int64 // optional: overrides the default file mode
}

// TestSymlinkTraversal_DefaultHandler verifies that extracting an archive
// containing a symlink with a relative traversal target is rejected.
func TestSymlinkTraversal_DefaultHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name: "relative symlink escapes destination",
			entries: []tarEntry{
				{name: "escape-link", linkTarget: "../../../../../../../etc/shadow"},
			},
		},
		{
			name: "absolute symlink target",
			entries: []tarEntry{
				{name: "abs-link", linkTarget: "/etc/shadow"},
			},
		},
		{
			name: "symlink to parent directory",
			entries: []tarEntry{
				{name: "parent-link", linkTarget: ".."},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, DecompressOpts{})
			require.Error(t, err, "extraction of archive with traversal symlink should fail")
		})
	}
}

// TestSymlinkTraversal_StripHandler verifies that the strip-components path
// also rejects symlinks that escape the destination.
func TestSymlinkTraversal_StripHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name: "relative symlink escapes after strip",
			entries: []tarEntry{
				{name: "prefix/escape-link", linkTarget: "../../../../etc/shadow"},
			},
		},
		{
			name: "absolute symlink target with strip",
			entries: []tarEntry{
				{name: "prefix/abs-link", linkTarget: "/etc/shadow"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, DecompressOpts{StripComponents: 1})
			require.Error(t, err, "extraction with strip should reject traversal symlinks")
		})
	}
}

// TestSymlinkTraversal_FilterHandler verifies that the filtered extraction
// path also rejects symlinks that escape the destination.
func TestSymlinkTraversal_FilterHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		files   []string
	}{
		{
			name: "relative symlink escapes in filtered extraction",
			entries: []tarEntry{
				{name: "escape-link", linkTarget: "../../../etc/shadow"},
			},
			files: []string{"escape-link"},
		},
		{
			name: "absolute symlink target in filtered extraction",
			entries: []tarEntry{
				{name: "abs-link", linkTarget: "/etc/shadow"},
			},
			files: []string{"abs-link"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "malicious.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, DecompressOpts{
				Files:          tc.files,
				SkipValidation: true,
			})
			require.Error(t, err, "filtered extraction should reject traversal symlinks")
		})
	}
}

// TestNameTraversal_ZipSlip verifies that archive entries with path traversal
// in NameInArchive (classic Zip Slip) are rejected across all handlers.
func TestNameTraversal_ZipSlip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		opts    DecompressOpts
	}{
		{
			name:    "default handler rejects traversal in name",
			entries: []tarEntry{{name: "../../etc/malicious.txt", content: "pwned"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "strip handler rejects traversal in name",
			entries: []tarEntry{{name: "prefix/../../etc/malicious.txt", content: "pwned"}},
			opts:    DecompressOpts{StripComponents: 1, OverwriteExisting: true},
		},
		{
			name:    "filter handler rejects traversal in name",
			entries: []tarEntry{{name: "../../etc/malicious.txt", content: "pwned"}},
			opts:    DecompressOpts{Files: []string{"../../etc/malicious.txt"}, SkipValidation: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "zipslip.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, tc.opts)
			require.Error(t, err, "extraction should reject entries with path traversal in name")

			// Verify the malicious file was NOT created anywhere under dst
			entries, readErr := os.ReadDir(dst)
			require.NoError(t, readErr)
			require.Empty(t, entries, "destination directory should be empty after rejected extraction")
		})
	}
}

// TestSymlinkOrderingAttack verifies that a symlink entry cannot redirect a
// subsequent entry outside the destination directory. This is a TOCTOU-style
// attack where the archive is crafted so a symlink is extracted first, then
// a later entry writes through that symlink to escape dst.
func TestSymlinkOrderingAttack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		opts    DecompressOpts
	}{
		{
			name: "symlink then file through symlink (default handler)",
			entries: []tarEntry{
				{name: "evil", linkTarget: ".."},
				{name: "evil/escape.txt", content: "pwned"},
			},
			opts: DecompressOpts{},
		},
		{
			name: "symlink then file through symlink (strip handler)",
			entries: []tarEntry{
				{name: "prefix/evil", linkTarget: ".."},
				{name: "prefix/evil/escape.txt", content: "pwned"},
			},
			opts: DecompressOpts{StripComponents: 1},
		},
		{
			name: "symlink then file through symlink (filter handler)",
			entries: []tarEntry{
				{name: "evil", linkTarget: ".."},
				{name: "evil/escape.txt", content: "pwned"},
			},
			opts: DecompressOpts{
				Files:          []string{"evil", "evil/escape.txt"},
				SkipValidation: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "ordering-attack.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, tc.opts)
			require.Error(t, err, "symlink ordering attack should be rejected")

			// Verify no file was written outside dst
			_, statErr := os.Stat(filepath.Join(dst, "..", "escape.txt"))
			require.True(t, os.IsNotExist(statErr), "file should not have been written outside dst")
		})
	}
}

// This is so windows tests can pass even when they don't have permission to create a symlink
func skipIfNoSymlink(t *testing.T) {
	t.Helper()
	err := os.Symlink("target", filepath.Join(t.TempDir(), "testlink"))
	if err != nil {
		t.Skipf("skipping: cannot create symlinks: %v", err)
	}
}

// TestSafeSymlinksAllowed verifies that symlinks staying within the destination
// directory are still permitted after the security fix is applied.
func TestSafeSymlinksAllowed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	skipIfNoSymlink(t)

	archivePath := filepath.Join(t.TempDir(), "safe.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "real-file.txt", content: "hello"},
		{name: "safe-link", linkTarget: "real-file.txt"},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{})
	require.NoError(t, err, "safe symlinks within destination should be allowed")

	// Verify the symlink was created and resolves to the correct content
	linkPath := filepath.Join(dst, "safe-link")
	info, err := os.Lstat(linkPath)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink, "expected a symlink")

	content, err := os.ReadFile(linkPath)
	require.NoError(t, err)
	require.Equal(t, "hello", string(content))
}

// TestSafeSymlinksAllowed_Subdirectory verifies that symlinks targeting files
// in subdirectories within the destination are permitted.
func TestSafeSymlinksAllowed_Subdirectory(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	skipIfNoSymlink(t)

	archivePath := filepath.Join(t.TempDir(), "safe-subdir.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "subdir/", isDir: true},
		{name: "subdir/file.txt", content: "nested"},
		{name: "link-to-nested", linkTarget: "subdir/file.txt"},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{})
	require.NoError(t, err, "symlinks to subdirectory files should be allowed")
}

// TestSafeSymlinksAllowed_StripHandler verifies that legitimate relative
// symlinks within subdirectories are permitted when using StripComponents.
// This catches an incorrect implementation that validates linkTarget relative
// to dst instead of relative to the symlink's parent directory.
func TestSafeSymlinksAllowed_StripHandler(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	skipIfNoSymlink(t)

	// Archive layout:
	//   prefix/deep/file.txt          (regular file)
	//   prefix/deep/nested/link       (symlink -> ../file.txt)
	//
	// After strip=1:
	//   deep/file.txt
	//   deep/nested/link -> ../file.txt
	//
	// The symlink resolves: dst/deep/nested/ + ../file.txt = dst/deep/file.txt
	// This is safely within dst and must be allowed.
	archivePath := filepath.Join(t.TempDir(), "safe-strip.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "prefix/", isDir: true},
		{name: "prefix/deep/", isDir: true},
		{name: "prefix/deep/file.txt", content: "safe content"},
		{name: "prefix/deep/nested/", isDir: true},
		{name: "prefix/deep/nested/link", linkTarget: "../file.txt"},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{StripComponents: 1})
	require.NoError(t, err, "relative symlink within dst should be allowed with strip")

	linkPath := filepath.Join(dst, "deep", "nested", "link")
	target, err := os.Readlink(linkPath)
	require.NoError(t, err, "symlink should exist")
	require.Equal(t, filepath.Join("..", "file.txt"), target)

	// Verify the symlink resolves to the correct content
	content, err := os.ReadFile(linkPath)
	require.NoError(t, err)
	require.Equal(t, "safe content", string(content))
}

// TestSafeSymlinksAllowed_FilterHandler verifies that legitimate symlinks
// are permitted when using filtered extraction.
func TestSafeSymlinksAllowed_FilterHandler(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	skipIfNoSymlink(t)

	archivePath := filepath.Join(t.TempDir(), "safe-filter.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "real-file.txt", content: "filtered content"},
		{name: "safe-link", linkTarget: "real-file.txt"},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{
		Files:          []string{"real-file.txt", "safe-link"},
		SkipValidation: true,
	})
	require.NoError(t, err, "safe symlinks should be allowed in filtered extraction")

	// Verify the symlink resolves to the correct content
	content, err := os.ReadFile(filepath.Join(dst, "safe-link"))
	require.NoError(t, err)
	require.Equal(t, "filtered content", string(content))
}

// TestWindowsReservedNames verifies that archive entries using Windows
// reserved device names are rejected on all platforms.
func TestWindowsReservedNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		opts    DecompressOpts
	}{
		{
			name:    "CON device name in entry",
			entries: []tarEntry{{name: "CON", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "NUL device name in subdirectory",
			entries: []tarEntry{{name: "subdir/NUL", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "COM1 device name",
			entries: []tarEntry{{name: "COM1", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "LPT1 device name",
			entries: []tarEntry{{name: "LPT1", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "PRN device name with strip",
			entries: []tarEntry{{name: "prefix/PRN", content: "data"}},
			opts:    DecompressOpts{StripComponents: 1},
		},
		{
			name:    "AUX device name in filtered extraction",
			entries: []tarEntry{{name: "AUX", content: "data"}},
			opts:    DecompressOpts{Files: []string{"AUX"}, SkipValidation: true},
		},
		{
			name:    "reserved name as symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "CON"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "COM0 device name",
			entries: []tarEntry{{name: "COM0", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "LPT0 device name",
			entries: []tarEntry{{name: "LPT0", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "NUL with extension",
			entries: []tarEntry{{name: "NUL.txt", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "COM1 with extension",
			entries: []tarEntry{{name: "COM1.tar.gz", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "reserved name with extension in subdirectory",
			entries: []tarEntry{{name: "subdir/AUX.log", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "reserved name with extension as symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "NUL.txt"}},
			opts:    DecompressOpts{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "reserved.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, tc.opts)
			require.Error(t, err, "entries with reserved device names should be rejected")
		})
	}
}

// TestColonInEntryName verifies that archive entries containing colons
// (drive letters, alternate data streams) are rejected on all platforms.
func TestColonInEntryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		opts    DecompressOpts
	}{
		{
			name:    "drive letter in entry name",
			entries: []tarEntry{{name: "C:file.txt", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "alternate data stream syntax",
			entries: []tarEntry{{name: "file.txt:hidden", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "drive-relative path",
			entries: []tarEntry{{name: "C:relative/file.txt", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "colon in symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "C:file.txt"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "colon with strip handler",
			entries: []tarEntry{{name: "prefix/file:stream", content: "data"}},
			opts:    DecompressOpts{StripComponents: 1},
		},
		{
			name:    "colon with filter handler",
			entries: []tarEntry{{name: "file:stream", content: "data"}},
			opts:    DecompressOpts{Files: []string{"file:stream"}, SkipValidation: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "colon.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, tc.opts)
			require.Error(t, err, "entries with colons should be rejected")
		})
	}
}

// TestTrailingDotsAndSpaces verifies that archive entries with trailing
// dots or spaces are rejected (Windows normalizes these silently).
func TestTrailingDotsAndSpaces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		opts    DecompressOpts
	}{
		{
			name:    "trailing dot in entry name",
			entries: []tarEntry{{name: "file.txt.", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "trailing space in entry name",
			entries: []tarEntry{{name: "file.txt ", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "trailing dots in subdirectory component",
			entries: []tarEntry{{name: "subdir./file.txt", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name: "trailing dot in symlink target",
			entries: []tarEntry{
				{name: "real.txt", content: "data"},
				{name: "link", linkTarget: "real.txt."},
			},
			opts: DecompressOpts{},
		},
		{
			name:    "trailing dot with strip handler",
			entries: []tarEntry{{name: "prefix/file.txt.", content: "data"}},
			opts:    DecompressOpts{StripComponents: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "trailing.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, tc.opts)
			require.Error(t, err, "entries with trailing dots or spaces should be rejected")
		})
	}
}

// TestBackslashInEntryName verifies that archive entries containing
// backslashes are rejected — backslashes are not POSIX-compliant in tar
// entry names and act as path separators on Windows.
func TestBackslashInEntryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
		opts    DecompressOpts
	}{
		{
			name:    "backslash in entry name",
			entries: []tarEntry{{name: "sub\\file.txt", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "backslash traversal in entry name",
			entries: []tarEntry{{name: "prefix\\..\\..\\escape.txt", content: "data"}},
			opts:    DecompressOpts{},
		},
		{
			name:    "backslash with strip handler",
			entries: []tarEntry{{name: "prefix/sub\\file.txt", content: "data"}},
			opts:    DecompressOpts{StripComponents: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "backslash.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, tc.opts)
			require.Error(t, err, "entries with backslashes should be rejected")
		})
	}
}

// TestRootedSymlinkTarget verifies that symlink targets starting with
// / or \ (rooted paths without a drive letter) are rejected.
func TestRootedSymlinkTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name:    "forward-slash rooted symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "/etc/passwd"}},
		},
		{
			name:    "backslash rooted symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "\\Windows\\System32"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "rooted.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, DecompressOpts{})
			require.Error(t, err, "rooted symlink targets should be rejected")
		})
	}
}

// TestVolumeNameInSymlinkTarget verifies that symlink targets containing
// Windows volume names (drive letters, UNC paths) are rejected.
func TestVolumeNameInSymlinkTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []tarEntry
	}{
		{
			name:    "drive letter in symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "C:file.txt"}},
		},
		{
			name:    "drive-absolute symlink target",
			entries: []tarEntry{{name: "link", linkTarget: "C:\\Windows\\System32"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "volume.tar.gz")
			makeTarGz(t, archivePath, tc.entries)

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, DecompressOpts{})
			require.Error(t, err, "symlink targets with volume names should be rejected")
		})
	}
}

// TestSetuidFileExtraction verifies that archive entries with setuid, setgid,
// or sticky bits are extracted successfully. os.Root.OpenFile rejects mode bits
// beyond 0o777, so writeFile must mask with Perm().
func TestSetuidFileExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode int64
	}{
		{name: "setuid bit", mode: 0o4755},
		{name: "setgid bit", mode: 0o2755},
		{name: "sticky bit", mode: 0o1755},
		{name: "setuid and setgid", mode: 0o6755},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			archivePath := filepath.Join(t.TempDir(), "setuid.tar.gz")
			makeTarGz(t, archivePath, []tarEntry{
				{name: "special-file", content: "data", mode: tc.mode},
			})

			dst := t.TempDir()
			err := Decompress(ctx, archivePath, dst, DecompressOpts{})
			require.NoError(t, err, "files with special mode bits should extract successfully")

			content, err := os.ReadFile(filepath.Join(dst, "special-file"))
			require.NoError(t, err)
			require.Equal(t, "data", string(content))
		})
	}
}

// TestHardlinkWithinRoot verifies that legitimate hardlinks within the
// destination directory are extracted correctly as hardlinks (not symlinks).
func TestHardlinkWithinRoot(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	archivePath := filepath.Join(t.TempDir(), "hardlink.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "original.txt", content: "shared content"},
		{name: "hardlink.txt", linkTarget: "original.txt", isHardLink: true},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{})
	require.NoError(t, err, "hardlinks within dst should be allowed")

	// Verify both files have the same content
	orig, err := os.ReadFile(filepath.Join(dst, "original.txt"))
	require.NoError(t, err)
	link, err := os.ReadFile(filepath.Join(dst, "hardlink.txt"))
	require.NoError(t, err)
	require.Equal(t, string(orig), string(link))

	// Verify they share the same inode (are actual hardlinks)
	origInfo, err := os.Stat(filepath.Join(dst, "original.txt"))
	require.NoError(t, err)
	linkInfo, err := os.Stat(filepath.Join(dst, "hardlink.txt"))
	require.NoError(t, err)
	require.True(t, os.SameFile(origInfo, linkInfo), "expected hardlink (same inode), got separate files")
}

// TestHardlinkEscapeRejected verifies that a hardlink targeting a file
// outside the destination directory is rejected by os.Root.Link.
func TestHardlinkEscapeRejected(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	archivePath := filepath.Join(t.TempDir(), "hardlink-escape.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "escape-link", linkTarget: "../../../etc/passwd", isHardLink: true},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{})
	require.Error(t, err, "hardlinks escaping dst should be rejected")
}

// TestHardlinkWithStripHandler verifies that hardlinks work correctly
// when using StripComponents.
func TestHardlinkWithStripHandler(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	archivePath := filepath.Join(t.TempDir(), "hardlink-strip.tar.gz")
	makeTarGz(t, archivePath, []tarEntry{
		{name: "prefix/original.txt", content: "stripped content"},
		{name: "prefix/hardlink.txt", linkTarget: "prefix/original.txt", isHardLink: true},
	})

	dst := t.TempDir()
	err := Decompress(ctx, archivePath, dst, DecompressOpts{StripComponents: 1})
	require.NoError(t, err, "hardlinks with strip should be allowed")

	content, err := os.ReadFile(filepath.Join(dst, "hardlink.txt"))
	require.NoError(t, err)
	require.Equal(t, "stripped content", string(content))
}

// TestMaliciousNestedArchive verifies that a clean outer archive containing
// a malicious inner .tar with traversal entries is rejected when using
// UnarchiveAll.
func TestMaliciousNestedArchive(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Step 1: Create the malicious inner .tar archive.
	innerDir := t.TempDir()
	innerPath := filepath.Join(innerDir, "evil.tar")
	innerF, err := os.Create(innerPath)
	require.NoError(t, err)
	innerTw := tar.NewWriter(innerF)
	require.NoError(t, innerTw.WriteHeader(&tar.Header{
		Name:     "escape-link",
		Typeflag: tar.TypeSymlink,
		Linkname: "../../../etc/shadow",
	}))
	require.NoError(t, innerTw.Close())
	require.NoError(t, innerF.Close())

	// Step 2: Create the outer tar.gz containing the inner .tar.
	innerData, err := os.ReadFile(innerPath)
	require.NoError(t, err)
	outerPath := filepath.Join(t.TempDir(), "outer.tar.gz")
	outerF, err := os.Create(outerPath)
	require.NoError(t, err)
	gw := gzip.NewWriter(outerF)
	outerTw := tar.NewWriter(gw)
	require.NoError(t, outerTw.WriteHeader(&tar.Header{
		Name:     "evil.tar",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(innerData)),
	}))
	_, err = outerTw.Write(innerData)
	require.NoError(t, err)
	require.NoError(t, outerTw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, outerF.Close())

	// Step 3: Extract with UnarchiveAll — should fail on the inner archive.
	dst := t.TempDir()
	err = Decompress(ctx, outerPath, dst, DecompressOpts{UnarchiveAll: true})
	require.Error(t, err, "malicious nested archive should be rejected")
}

func TestValidateEntryName(t *testing.T) {
	t.Parallel()

	valid := []string{
		"file.txt",
		"sub/dir/file.txt",
		"a/b/c",
		".hidden",
		"sub/.hidden/file",
	}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, validateEntryName(name))
		})
	}

	invalid := []struct {
		name     string
		input    string
		contains string
	}{
		{"empty", "", "empty"},
		{"backslash", "sub\\file.txt", "backslash"},
		{"colon drive letter", "C:file.txt", "':'"},
		{"colon ADS", "file.txt:hidden", "':'"},
		{"trailing dot", "file.txt.", "trailing dots"},
		{"trailing space", "file.txt ", "trailing dots or spaces"},
		{"reserved CON", "CON", "reserved device name"},
		{"reserved NUL in subdir", "sub/NUL", "reserved device name"},
		{"reserved COM0", "COM0", "reserved device name"},
		{"reserved LPT0", "LPT0", "reserved device name"},
		{"reserved NUL with extension", "NUL.txt", "reserved device name"},
		{"reserved COM1 with extension", "COM1.tar.gz", "reserved device name"},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateEntryName(tc.input)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.contains)
		})
	}
}

func TestValidateSymlink(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name       string
		rel        string
		linkTarget string
	}{
		{"same directory", "link", "target.txt"},
		{"subdirectory target", "link", "sub/target.txt"},
		{"parent within root", "sub/link", "../target.txt"},
		{"deep nesting", "a/b/c/link", "../../d/target.txt"},
	}
	for _, tc := range valid {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, validateSymlink(tc.rel, tc.linkTarget))
		})
	}

	invalid := []struct {
		name       string
		rel        string
		linkTarget string
		contains   string
	}{
		{"empty target", "link", "", "empty symlink target"},
		{"absolute target", "link", "/etc/passwd", "absolute"},
		{"escape via dotdot", "link", "../escape", "escapes root"},
		{"deep escape", "sub/link", "../../escape", "escapes root"},
		{"rooted backslash", "link", "\\Windows\\System32", "rooted"},
	}
	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateSymlink(tc.rel, tc.linkTarget)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.contains)
		})
	}

	// filepath.VolumeName only parses drive letters on Windows, so the
	// volume-name check in validateSymlink is only effective there.
	t.Run("invalid/volume_name", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "windows" {
			t.Skip("filepath.VolumeName does not detect drive letters on non-Windows")
		}
		err := validateSymlink("link", "C:file.txt")
		require.Error(t, err)
		require.ErrorContains(t, err, "volume name")
	})
}
