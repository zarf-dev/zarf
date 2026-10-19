// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

// TestWriteTarUnpacksAsAnImageArchive builds a volume, writes it out, and
// pulls it back in through the imageArchives path packages use. That is the
// real consumer of what WriteTar produces, so it checks the archive layout
// end to end rather than only the entry names.
func TestWriteTarUnpacksAsAnImageArchive(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	const ref = "zarf.internal/docs:local"

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "index.html"), []byte("<h1>offline</h1>"), 0o644))

	iv := newTestVolume(t)
	require.NoError(t, iv.AddDirectory(ctx, srcDir, ref))

	tarPath := filepath.Join(t.TempDir(), "docs.tar")
	out, err := os.Create(tarPath)
	require.NoError(t, err)
	require.NoError(t, iv.WriteTar(ctx, ref, out))
	require.NoError(t, out.Close())

	pulled, err := images.Unpack(ctx, v1alpha1.ImageArchive{
		Path:   tarPath,
		Images: []string{ref},
	}, t.TempDir(), string(PlatformArchAMD64))
	require.NoError(t, err)
	require.Len(t, pulled, 1)
	require.Equal(t, ref, pulled[0].Image.Reference)
}
