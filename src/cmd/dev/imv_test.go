// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package dev

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/zoci/archive"
	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestNewImageVolumeCommand(t *testing.T) {
	t.Parallel()

	cmd := NewImageVolumeCommand()

	require.Equal(t, "image-volume-archive [ DIRECTORY ] [ IMAGE-REFERENCE ]", cmd.Use)
	require.Equal(t, []string{"iva"}, cmd.Aliases)
	require.NotNil(t, cmd.PreRunE)
	require.NotNil(t, cmd.RunE)

	compression, err := cmd.Flags().GetString("layer-compression")
	require.NoError(t, err)
	require.Equal(t, string(image.VolumeCompressionGzip), compression)

	platformOS, err := cmd.Flags().GetString("platform-os")
	require.NoError(t, err)
	require.Equal(t, string(image.PlatformOSLinux), platformOS)

	output, err := cmd.Flags().GetString("output")
	require.NoError(t, err)
	require.Empty(t, output)
}

func TestImageVolumeOptionsPrerun(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		o          *imageVolumeOptions
		args       []string
		wantOutput string
		wantErr    error
	}{
		{
			name:       "derives output from ref when unset",
			o:          &imageVolumeOptions{compression: image.VolumeCompressionGzip, os: image.PlatformOSLinux},
			args:       []string{"dir", "localhost:5000/rpm:latest"},
			wantOutput: "localhost-5000_rpm-latest.tar",
		},
		{
			name:       "keeps explicit output",
			o:          &imageVolumeOptions{compression: image.VolumeCompressionGzip, os: image.PlatformOSLinux, output: "custom.tar"},
			args:       []string{"dir", "rpm:latest"},
			wantOutput: "custom.tar",
		},
		{
			name:    "explicit output must end in .tar",
			o:       &imageVolumeOptions{compression: image.VolumeCompressionGzip, os: image.PlatformOSLinux, output: "custom.zip"},
			args:    []string{"dir", "rpm:latest"},
			wantErr: archive.ErrNotTarBall,
		},
		{
			name:    "invalid compression",
			o:       &imageVolumeOptions{compression: image.VolumeCompression("bogus"), os: image.PlatformOSLinux},
			args:    []string{"dir", "rpm:latest"},
			wantErr: image.ErrLayerCompression,
		},
		{
			name:    "invalid platform os",
			o:       &imageVolumeOptions{compression: image.VolumeCompressionGzip, os: image.PlatformOS("plan9")},
			args:    []string{"dir", "rpm:latest"},
			wantErr: image.ErrPlatformOS,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.o.prerun(nil, tc.args)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantOutput, tc.o.output)
		})
	}
}

func TestImageVolumeOptionsRun(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("hello world"), 0o644))

	out := filepath.Join(t.TempDir(), "out.tar")
	o := &imageVolumeOptions{
		compression: image.VolumeCompressionUncompressed,
		os:          image.PlatformOSLinux,
		output:      out,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := o.run(cmd, []string{srcDir, "test:latest"})
	require.NoError(t, err)
	require.FileExists(t, out)

	f, err := os.Open(out)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	names := map[string]bool{}
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		names[hdr.Name] = true
	}
	require.True(t, names["oci-layout"], "expected oci-layout entry")
	require.True(t, names["index.json"], "expected index.json entry")
}

func TestImageVolumeOptionsRunInvalidDirectory(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	o := &imageVolumeOptions{
		compression: image.VolumeCompressionUncompressed,
		os:          image.PlatformOSLinux,
		output:      filepath.Join(t.TempDir(), "out.tar"),
	}

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := o.run(cmd, []string{filepath.Join(t.TempDir(), "does-not-exist"), "test:latest"})
	require.Error(t, err)
}
