// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChartArchiveName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "podinfo-6.4.0.tgz", ChartArchiveName("podinfo", "6.4.0"))
	require.Equal(t, "podinfo.tgz", ChartArchiveName("podinfo", ""),
		"a versionless chart keeps its bare name")
}

func TestChartValuesFileName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "podinfo-6.4.0-0", ChartValuesFileName("podinfo", "6.4.0", 0))
	require.Equal(t, "podinfo-6.4.0-3", ChartValuesFileName("podinfo", "6.4.0", 3))
	require.Equal(t, "podinfo-0", ChartValuesFileName("podinfo", "", 0))

	// Values files share the archive's stem but carry no extension, so the two
	// names must not be derived from one another.
	require.NotContains(t, ChartValuesFileName("podinfo", "6.4.0", 0), ".tgz")
}

func TestChartPaths(t *testing.T) {
	t.Parallel()

	paths := ChartPaths{ChartsDir: filepath.Join("build", "charts"), ValuesDir: filepath.Join("build", "values")}

	require.Equal(t, filepath.Join("build", "charts", "podinfo-6.4.0.tgz"), paths.Archive("podinfo", "6.4.0"))
	require.Equal(t, filepath.Join("build", "values", "podinfo-6.4.0-1"), paths.ValuesFile("podinfo", "6.4.0", 1))
}

func TestManifestFileNames(t *testing.T) {
	t.Parallel()

	require.Equal(t, "my-manifest-0.yaml", ManifestFileName("my-manifest", 0))
	require.Equal(t, "kustomization-my-manifest-2.yaml", KustomizationFileName("my-manifest", 2))
}

func TestComponentFileRelPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		idx    int
		target string
		want   string
	}{
		{name: "POSIX path", idx: 0, target: "/etc/app/file.txt", want: filepath.Join("0", "file.txt")},
		{name: "Windows path", idx: 1, target: `C:\app\file.txt`, want: filepath.Join("1", "file.txt")},
		{name: "Windows path with forward slashes", idx: 2, target: "C:/app/file.txt", want: filepath.Join("2", "file.txt")},
		{name: "file name", idx: 3, target: "file.txt", want: filepath.Join("3", "file.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ComponentFileRelPath(tt.idx, tt.target))
		})
	}
}
