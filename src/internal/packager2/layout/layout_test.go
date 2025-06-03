// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/internal/packager2/load"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestCreateSkeleton(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")
	pkg, err := load.PackageDefinition(ctx, "./testdata/zarf-skeleton-package", load.DefinitionOpts{})
	require.NoError(t, err)

	opt := layout.SkeletonLayoutOptions{}
	pkgLayout, err := layout.AssembleSkeleton(ctx, pkg, "./testdata/zarf-skeleton-package", opt)
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(pkgLayout.DirPath(), "checksums.txt"))
	require.NoError(t, err)
	expectedChecksum := `0fea7403536c0c0e2a2d9b235d4b3716e86eefd8e78e7b14412dd5a750b77474 components/kustomizations.tar
54f657b43323e1ebecb0758835b8d01a0113b61b7bab0f4a8156f031128d00f9 components/data-injections.tar
879bfe82d20f7bdcd60f9e876043cc4343af4177a6ee8b2660c304a5b6c70be7 components/files.tar
c497f1a56559ea0a9664160b32e4b377df630454ded6a3787924130c02f341a6 components/manifests.tar
fb7ebee94a4479bacddd71195030a483b0b0b96d4f73f7fcd2c2c8e0fce0c5c6 components/helm-charts.tar
`

	require.Equal(t, expectedChecksum, string(b))
}

func writePackageToDisk(t *testing.T, pkg v1alpha1.ZarfPackage, dir string) {
	t.Helper()
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	path := filepath.Join(dir, layout.ZarfYAML)
	err = os.WriteFile(path, b, 0700)
	require.NoError(t, err)
}

func TestGetSBOM(t *testing.T) {
	t.Parallel()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")

	ctx := testutil.TestContext(t)

	tmpdir := t.TempDir()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "test-sbom",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "do-nothing",
			},
		},
	}
	writePackageToDisk(t, pkg, tmpdir)
	// FIXME add
	pkg, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOpts{})
	require.NoError(t, err)

	pkgLayout, err := layout.AssemblePackage(ctx, pkg, tmpdir, layout.AssembleLayoutOptions{})
	require.NoError(t, err)

	// Ensure the SBOM does not exist
	require.NoFileExists(t, filepath.Join(pkgLayout.DirPath(), layout.SBOMTar))
	// Ensure Zarf errors correctly
	err = pkgLayout.GetSBOM(ctx, tmpdir)
	var noSBOMErr *layout.NoSBOMAvailableError
	require.ErrorAs(t, err, &noSBOMErr)
}

func TestCreateAbsolutePathFileSource(t *testing.T) {
	t.Parallel()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")
	ctx := testutil.TestContext(t)

	createFileToImport := func(t *testing.T, dir string) string {
		t.Helper()
		absoluteFilePath, err := filepath.Abs(filepath.Join(dir, "file.txt"))
		require.NoError(t, err)
		_, err = os.Create(absoluteFilePath)
		require.NoError(t, err)
		return absoluteFilePath
	}

	t.Run("test a standard package can use absolute file paths", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		absoluteFilePath := createFileToImport(t, tmpdir)
		pkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name: "standard",
			},
			Components: []v1alpha1.ZarfComponent{
				{
					Name: "file",
					Files: []v1alpha1.ZarfFile{
						{
							Source: absoluteFilePath,
							Target: "file.txt",
						},
					},
				},
			},
		}
		// Create the zarf.yaml file in the tmpdir
		writePackageToDisk(t, pkg, tmpdir)

		pkg, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOpts{})
		require.NoError(t, err)

		pkgLayout, err := layout.AssemblePackage(ctx, pkg, tmpdir, layout.AssembleLayoutOptions{})
		require.NoError(t, err)

		// Ensure the components have the correct file
		fileComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, "file", layout.FilesComponentDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(fileComponent, "0", "file.txt"))
	})

	t.Run("test that imports handle absolute paths properly", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		absoluteFilePath := createFileToImport(t, tmpdir)
		parentPkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name: "parent",
			},
			Components: []v1alpha1.ZarfComponent{
				{
					Name: "file-import",
					Import: v1alpha1.ZarfComponentImport{
						Path: "child",
					},
				},
			},
		}
		// Create package using absolute file path set to be import
		childPkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name: "child",
			},
			Components: []v1alpha1.ZarfComponent{
				{
					Name: "file-import",
					Files: []v1alpha1.ZarfFile{
						{
							Source: absoluteFilePath,
							Target: "file.txt",
						},
					},
				},
			},
		}
		// Create zarf.yaml files in the tempdir
		writePackageToDisk(t, parentPkg, tmpdir)
		childDir := filepath.Join(tmpdir, "child")
		err := os.Mkdir(childDir, 0700)
		require.NoError(t, err)
		writePackageToDisk(t, childPkg, childDir)
		pkg, err := load.PackageDefinition(ctx, tmpdir, load.DefinitionOpts{})
		require.NoError(t, err)
		// create the package
		pkgLayout, err := layout.AssemblePackage(context.Background(), pkg, tmpdir, layout.AssembleLayoutOptions{})
		require.NoError(t, err)

		// Ensure the component has the correct file
		importedFileComponent, err := pkgLayout.GetComponentDir(ctx, tmpdir, "file-import", layout.FilesComponentDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(importedFileComponent, "0", "file.txt"))
	})
}
