// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package dev

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/cmd/completion"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/zoci/archive"
	"github.com/zarf-dev/zarf/src/pkg/zoci/image"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestNewImageVolumeCommand(t *testing.T) {
	t.Parallel()

	cmd := NewImageVolumeCommand()

	require.Equal(t, lang.CmdDevImageVolumeArchiveUsage, cmd.Use)
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

	maxLayers, err := cmd.Flags().GetUint8("max-layers")
	require.NoError(t, err)
	require.Equal(t, image.DefaultMaxLayers, maxLayers)
	require.NotNil(t, cmd.Flags().ShorthandLookup("m"), "max-layers should have the -m shorthand")

	// "-o" belongs to output, the way it does in every other CLI. Handing it
	// to --platform-os would silently write the archive somewhere the user did
	// not ask for.
	require.Equal(t, "output", cmd.Flags().ShorthandLookup("o").Name)
	require.Empty(t, cmd.Flags().Lookup("platform-os").Shorthand)
	require.Equal(t, "layer-compression", cmd.Flags().ShorthandLookup("c").Name)
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

func TestImageVolumeOptionsRunBatchesWithinMaxLayers(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "b.txt"), []byte("b"), 0o644))

	out := filepath.Join(t.TempDir(), "out.tar")
	o := &imageVolumeOptions{
		compression: image.VolumeCompressionUncompressed,
		os:          image.PlatformOSLinux,
		output:      out,
		maxLayers:   1,
	}

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	require.NoError(t, o.run(cmd, []string{srcDir, "test:latest"}))
	require.FileExists(t, out)

	// A plumbing bug that silently drops o.maxLayers wouldn't fail here: the
	// resulting image.Options would carry no cap at all, which image.New
	// reads as DefaultMaxLayers, well above the two files. Only counting the
	// actual layers in the output proves maxLayers reached the Volume.
	require.Len(t, manifestLayers(t, out), 1)
}

// TestImageVolumeOptionsImageOptions pins the one flag that cannot be passed
// through as-is: --max-layers spells "no cap" as 0, while image.Options
// reserves 0 for "unset" and carries an absent cap as UnlimitedLayers. Setting
// both of those is an error, so the mapping has to pick exactly one.
func TestImageVolumeOptionsImageOptions(t *testing.T) {
	t.Parallel()

	unlimited := (&imageVolumeOptions{maxLayers: completion.UnlimitedMaxLayers}).imageOptions()
	require.True(t, unlimited.UnlimitedLayers)
	require.Zero(t, unlimited.MaxLayers)

	capped := (&imageVolumeOptions{maxLayers: 42}).imageOptions()
	require.False(t, capped.UnlimitedLayers)
	require.Equal(t, uint8(42), capped.MaxLayers)

	// The rest are carried straight over.
	o := &imageVolumeOptions{os: image.PlatformOSWindows, compression: image.VolumeCompressionZstd}
	opts := o.imageOptions()
	require.Equal(t, image.PlatformOSWindows, opts.OS)
	require.Equal(t, image.VolumeCompressionZstd, opts.Compression)
	require.Equal(t, image.PlatformArch(config.GetArch()), opts.Arch)
}

// TestImageVolumeOptionsImageOptionsAcceptsCompletions checks that every
// --max-layers value tab completion suggests survives the translation above
// and builds a Volume, so completion cannot offer a value the command rejects.
func TestImageVolumeOptionsImageOptionsAcceptsCompletions(t *testing.T) {
	t.Parallel()

	for _, line := range completion.ImageVolumeMaxLayers() {
		value, _, _ := strings.Cut(line, "\t")
		n, err := strconv.ParseUint(value, 10, 8)
		require.NoError(t, err, "suggested %q should parse as a uint8", value)

		o := &imageVolumeOptions{os: image.PlatformOSLinux, maxLayers: uint8(n)}
		iv, err := image.New(t.TempDir(), o.imageOptions())
		require.NoError(t, err, "suggested %q", value)
		require.NoError(t, iv.Clean())
	}
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

// manifestLayers reads the Docker/OCI-compatible tar archive at tarPath and
// returns the layer list of the single image manifest it contains.
func manifestLayers(t *testing.T, tarPath string) []ocispec.Descriptor {
	t.Helper()

	f, err := os.Open(tarPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	blobs := map[string][]byte{}
	var index ocispec.Index
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		data, err := io.ReadAll(tr)
		require.NoError(t, err)

		parts := strings.Split(hdr.Name, "/")
		switch {
		case hdr.Name == "index.json":
			require.NoError(t, json.Unmarshal(data, &index))
		case hdr.Typeflag == tar.TypeReg && strings.HasPrefix(hdr.Name, "blobs/") && len(parts) == 3:
			blobs[parts[1]+":"+parts[2]] = data
		}
	}

	require.Len(t, index.Manifests, 1, "expected a single image manifest in the archive")
	manifestBlob, ok := blobs[index.Manifests[0].Digest.String()]
	require.True(t, ok, "manifest blob missing from archive")

	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBlob, &manifest))
	return manifest.Layers
}
