// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageLayout(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	pathToPackage := filepath.Join("..", "testdata", "load-package", "compressed")

	pkgLayout, err := LoadFromTar(ctx, filepath.Join(pathToPackage, "zarf-package-test-amd64-0.0.1.tar.zst"), PackageLayoutOptions{})
	require.NoError(t, err)

	require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
	require.Equal(t, "0.0.1", pkgLayout.Pkg.Metadata.Version)

	tmpDir := t.TempDir()
	manifestDir, err := pkgLayout.GetComponentDir(ctx, tmpDir, "test", ManifestsComponentDir)
	require.NoError(t, err)
	expected, err := os.ReadFile(filepath.Join(pathToPackage, "deployment.yaml"))
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join(manifestDir, "deployment-0.yaml"))
	require.NoError(t, err)
	require.Equal(t, expected, b)

	_, err = pkgLayout.GetComponentDir(ctx, t.TempDir(), "does-not-exist", ManifestsComponentDir)
	require.ErrorContains(t, err, "component does-not-exist does not exist in package")

	_, err = pkgLayout.GetComponentDir(ctx, t.TempDir(), "test", FilesComponentDir)
	require.ErrorContains(t, err, "component test could not access a files directory")

	tmpDir = t.TempDir()
	err = pkgLayout.GetSBOM(ctx, tmpDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(tmpDir, "compare.html"))

	files, err := pkgLayout.Files()
	require.NoError(t, err)
	expectedNames := []string{
		"checksums.txt",
		"components/test.tar",
		"images/blobs/sha256/43180c492a5e6cedd8232e8f77a454f666f247586853eecb90258b26688ad1d3",
		"images/blobs/sha256/ff221270b9fb7387b0ad9ff8f69fbbd841af263842e62217392f18c3b5226f38",
		"images/blobs/sha256/0a9a5dfd008f05ebc27e4790db0709a29e527690c21bcbcd01481eaeb6bb49dc",
		"images/index.json",
		"images/oci-layout",
		"sboms.tar",
		"zarf.yaml",
	}
	require.Len(t, expectedNames, len(files))
	for _, expectedName := range expectedNames {
		path := filepath.Join(pkgLayout.dirPath, filepath.FromSlash(expectedName))
		name := files[path]
		require.Equal(t, expectedName, name)
	}
}

func TestPackageFileName(t *testing.T) {
	t.Parallel()
	config.CLIArch = "amd64"
	tests := []struct {
		name        string
		pkg         v1alpha1.ZarfPackage
		expected    string
		expectedErr string
	}{
		{
			name: "no architecture",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Version: "v0.55.4",
				},
			},
			expectedErr: "package must include a build architecture",
		},
		{
			name: "init package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-init-amd64-v0.55.4.tar.zst",
		},
		{
			name: "init package with a custom name",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
					Flavor:       "upstream",
				},
			},
			expected: "zarf-init-amd64-v0.55.4-upstream.tar.zst",
		},
		{
			name: "regular package with version",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name:    "my-package",
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-package-my-package-amd64-v0.55.4.tar.zst",
		},
		{
			name: "regular package no version",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "my-package",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-package-my-package-amd64.tar.zst",
		},
		{
			name: "differential package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name:    "my-package",
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Differential:               true,
					Architecture:               "amd64",
					DifferentialPackageVersion: "v0.55.3",
				},
			},
			expected: "zarf-package-my-package-amd64-v0.55.3-differential-v0.55.4.tar.zst",
		},
		{
			name: "flavor package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name:    "my-package",
					Version: "v0.55.4",
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
					Flavor:       "upstream",
				},
			},
			expected: "zarf-package-my-package-amd64-v0.55.4-upstream.tar.zst",
		},
		{
			name: "uncompressed",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name:         "my-package",
					Version:      "v0.55.4",
					Uncompressed: true,
				},
				Build: v1alpha1.ZarfBuildData{
					Architecture: "amd64",
				},
			},
			expected: "zarf-package-my-package-amd64-v0.55.4.tar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			layout := PackageLayout{Pkg: tt.pkg}
			actual, err := layout.FileName()
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
			}
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestPackageLayoutSignPackage(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	t.Run("successful signing", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)
		require.FileExists(t, legacySignaturePath, "legacy signature should exist")
		require.NotNil(t, pkgLayout.Pkg.Build.Signed)
		require.True(t, *pkgLayout.Pkg.Build.Signed)
	})

	t.Run("wrong password", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		bundlePath := filepath.Join(tmpDir, Bundle)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("wrongpassword"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.ErrorContains(t, err, "failed to sign package")
		require.ErrorContains(t, err, "reading key: decrypt: encrypted: decryption failed")
		require.NoFileExists(t, bundlePath)
		require.NoFileExists(t, legacySignaturePath)
	})

	t.Run("missing zarf.yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err := pkgLayout.SignPackage(ctx, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot access zarf.yaml for signing")
	})

	t.Run("invalid directory path", func(t *testing.T) {
		pkgLayout := &PackageLayout{
			dirPath: "/nonexistent/path",
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err := pkgLayout.SignPackage(ctx, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid package layout directory")
	})

	t.Run("empty dirPath", func(t *testing.T) {
		pkgLayout := &PackageLayout{
			dirPath: "",
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err := pkgLayout.SignPackage(ctx, opts)
		require.EqualError(t, err, "invalid package layout: dirPath is empty")
	})

	t.Run("overwrite existing signature", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		// Create existing legacy signature file
		err = os.WriteFile(legacySignaturePath, []byte("old legacy signature"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc
		opts.Overwrite = true

		// Should overwrite the existing signature (with warning logged)
		err = pkgLayout.SignPackage(ctx, opts)

		require.NoError(t, err)
		require.FileExists(t, legacySignaturePath)

		// Verify the signature was overwritten (not the old content)
		legacyContent, err := os.ReadFile(legacySignaturePath)
		require.NoError(t, err)
		require.NotEqual(t, "old legacy signature", string(legacyContent))
	})

	t.Run("skip signing when ShouldSign returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		bundlePath := filepath.Join(tmpDir, Bundle)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Empty options - no signing key material configured
		opts := utils.SignBlobOptions{}

		// Should skip signing without error
		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)
		require.NoFileExists(t, bundlePath)
		require.NoFileExists(t, legacySignaturePath)
		require.Nil(t, pkgLayout.Pkg.Build.Signed)
	})

	t.Run("dirPath is file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "somefile.txt")
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: filePath,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not a directory")
	})

	t.Run("input options not mutated", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc
		opts.OutputSignature = "/some/custom/path.sig"

		// Store original value
		originalOutputSignature := opts.OutputSignature

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)

		// Verify input options were not modified
		require.Equal(t, originalOutputSignature, opts.OutputSignature)
		require.NotEqual(t, opts.OutputSignature, filepath.Join(tmpDir, Signature))
	})

	t.Run("Signed field not set on error", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Wrong password should cause signing to fail
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("wrongpassword"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.Error(t, err)

		// Verify Signed field was not set
		require.Nil(t, pkgLayout.Pkg.Build.Signed)
	})

	t.Run("preserves existing Signed value on skip", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		existingSigned := false
		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					Signed: &existingSigned,
				},
			},
		}

		// Empty options - should skip signing
		opts := utils.SignBlobOptions{}

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)

		// Verify Signed field preserved
		require.NotNil(t, pkgLayout.Pkg.Build.Signed)
		require.False(t, *pkgLayout.Pkg.Build.Signed)
	})

	t.Run("zarf.yaml updated with signed:true after signing", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		// Create initial zarf.yaml with a valid package
		initialPkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Build: v1alpha1.ZarfBuildData{
				Architecture: "amd64",
			},
		}

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     initialPkg,
		}

		// Marshal and write initial package (without signed field)
		b, err := goyaml.Marshal(initialPkg)
		require.NoError(t, err)
		err = os.WriteFile(yamlPath, b, 0o644)
		require.NoError(t, err)

		// Sign the package
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)

		// Verify only legacy signature exists (bundle disabled by default)
		legacySignaturePath := filepath.Join(tmpDir, Signature)
		require.FileExists(t, legacySignaturePath, "legacy signature should exist")

		// Read the zarf.yaml from disk
		updatedBytes, err := os.ReadFile(yamlPath)
		require.NoError(t, err)

		// Parse it back
		var updatedPkg v1alpha1.ZarfPackage
		err = goyaml.Unmarshal(updatedBytes, &updatedPkg)
		require.NoError(t, err)

		// Verify that signed:true is now in the file on disk
		require.NotNil(t, updatedPkg.Build.Signed, "zarf.yaml should contain signed field")
		require.True(t, *updatedPkg.Build.Signed, "zarf.yaml should have signed:true")

		// Also verify in-memory state matches
		require.NotNil(t, pkgLayout.Pkg.Build.Signed)
		require.True(t, *pkgLayout.Pkg.Build.Signed)
	})
}

// TestPackageLayoutSignPackageValidation uses table-driven tests for validation scenarios
func TestPackageLayoutSignPackageValidation(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) (*PackageLayout, utils.SignBlobOptions)
		expectedErr    string
		expectSigned   bool
		expectSignFile bool
	}{
		{
			name: "package with existing false Signed value gets updated on success",
			setupFunc: func(t *testing.T) (*PackageLayout, utils.SignBlobOptions) {
				tmpDir := t.TempDir()
				yamlPath := filepath.Join(tmpDir, ZarfYAML)
				require.NoError(t, os.WriteFile(yamlPath, []byte("foobar"), 0o644))

				existingSigned := false
				layout := &PackageLayout{
					dirPath: tmpDir,
					Pkg: v1alpha1.ZarfPackage{
						Build: v1alpha1.ZarfBuildData{
							Signed: &existingSigned,
						},
					},
				}

				passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
					return []byte("test"), nil
				})
				opts := utils.DefaultSignBlobOptions()
				opts.KeyRef = "./testdata/cosign.key"
				opts.PassFunc = passFunc

				return layout, opts
			},
			expectedErr:    "",
			expectSigned:   true,
			expectSignFile: true,
		},
		{
			name: "package with existing true Signed value gets overwritten",
			setupFunc: func(t *testing.T) (*PackageLayout, utils.SignBlobOptions) {
				tmpDir := t.TempDir()
				yamlPath := filepath.Join(tmpDir, ZarfYAML)
				require.NoError(t, os.WriteFile(yamlPath, []byte("foobar"), 0o644))

				existingSigned := true
				layout := &PackageLayout{
					dirPath: tmpDir,
					Pkg: v1alpha1.ZarfPackage{
						Build: v1alpha1.ZarfBuildData{
							Signed: &existingSigned,
						},
					},
				}

				passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
					return []byte("test"), nil
				})
				opts := utils.DefaultSignBlobOptions()
				opts.KeyRef = "./testdata/cosign.key"
				opts.PassFunc = passFunc

				return layout, opts
			},
			expectedErr:    "",
			expectSigned:   true,
			expectSignFile: true,
		},
		{
			name: "sign with different password-protected key",
			setupFunc: func(t *testing.T) (*PackageLayout, utils.SignBlobOptions) {
				tmpDir := t.TempDir()
				yamlPath := filepath.Join(tmpDir, ZarfYAML)
				require.NoError(t, os.WriteFile(yamlPath, []byte("test content"), 0o644))

				layout := &PackageLayout{
					dirPath: tmpDir,
					Pkg:     v1alpha1.ZarfPackage{},
				}

				passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
					return []byte("test"), nil
				})
				opts := utils.DefaultSignBlobOptions()
				opts.KeyRef = "./testdata/cosign.key"
				opts.PassFunc = passFunc

				return layout, opts
			},
			expectedErr:    "",
			expectSigned:   true,
			expectSignFile: true,
		},
		{
			name: "passFunc returns error",
			setupFunc: func(t *testing.T) (*PackageLayout, utils.SignBlobOptions) {
				tmpDir := t.TempDir()
				yamlPath := filepath.Join(tmpDir, ZarfYAML)
				require.NoError(t, os.WriteFile(yamlPath, []byte("foobar"), 0o644))

				layout := &PackageLayout{
					dirPath: tmpDir,
					Pkg:     v1alpha1.ZarfPackage{},
				}

				passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
					return nil, os.ErrPermission
				})
				opts := utils.DefaultSignBlobOptions()
				opts.KeyRef = "./testdata/cosign.key"
				opts.PassFunc = passFunc

				return layout, opts
			},
			expectedErr:    "permission denied",
			expectSigned:   false,
			expectSignFile: false,
		},
		{
			name: "empty package metadata still signs",
			setupFunc: func(t *testing.T) (*PackageLayout, utils.SignBlobOptions) {
				tmpDir := t.TempDir()
				yamlPath := filepath.Join(tmpDir, ZarfYAML)
				require.NoError(t, os.WriteFile(yamlPath, []byte("foobar"), 0o644))

				layout := &PackageLayout{
					dirPath: tmpDir,
					Pkg: v1alpha1.ZarfPackage{
						Metadata: v1alpha1.ZarfMetadata{},
						Build:    v1alpha1.ZarfBuildData{},
					},
				}

				passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
					return []byte("test"), nil
				})
				opts := utils.DefaultSignBlobOptions()
				opts.KeyRef = "./testdata/cosign.key"
				opts.PassFunc = passFunc

				return layout, opts
			},
			expectedErr:    "",
			expectSigned:   true,
			expectSignFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			layout, opts := tt.setupFunc(t)

			err := layout.SignPackage(ctx, opts)

			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				if !tt.expectSigned {
					// On error, Signed should not be set to true
					if layout.Pkg.Build.Signed != nil {
						require.False(t, *layout.Pkg.Build.Signed)
					}
				}
				return
			}

			require.NoError(t, err)

			if tt.expectSigned {
				require.NotNil(t, layout.Pkg.Build.Signed)
				require.True(t, *layout.Pkg.Build.Signed)
			}

			if tt.expectSignFile {
				signPath := filepath.Join(layout.dirPath, Signature)
				require.FileExists(t, signPath)
			}
		})
	}
}

func TestPackageLayoutVerifyPackageSignature(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	t.Run("successful verification with valid signature", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		// Create and sign a package
		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Sign the package (legacy only, bundle feature disabled by default)
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)
		require.FileExists(t, legacySignaturePath, "legacy signature should exist")

		// Verify the signature (should use legacy format)
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.NoError(t, err)
	})

	t.Run("verification fails with wrong public key", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		// Create and sign a package
		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Sign with the test key
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)

		// Try to verify with a different (non-existent) key - should fail
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/nonexistent.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
	})

	t.Run("verification fails when signature missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		// Create zarf.yaml but no signature
		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "a key was provided but the package is not signed")
	})

	t.Run("verification fails with empty dirPath", func(t *testing.T) {
		pkgLayout := &PackageLayout{
			dirPath: "",
			Pkg:     v1alpha1.ZarfPackage{},
		}

		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err := pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.EqualError(t, err, "invalid package layout: dirPath is empty")
	})

	t.Run("verification fails with invalid directory", func(t *testing.T) {
		pkgLayout := &PackageLayout{
			dirPath: "/nonexistent/path",
			Pkg:     v1alpha1.ZarfPackage{},
		}

		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err := pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid package layout directory")
	})

	t.Run("verification fails when dirPath is a file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "somefile.txt")
		err := os.WriteFile(filePath, []byte("content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: filePath,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is not a directory")
	})

	t.Run("verification fails with no public key", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		// Create signed package
		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Sign the package
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)

		// Try to verify without providing a key
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "" // Empty key

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.EqualError(t, err, "package is signed but no verification material was provided (Public Key, etc.)")
	})

	t.Run("verification fails when signature is corrupted", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		// Create and sign package
		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Sign the package
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)

		// Corrupt all signature files that were produced
		for _, sigFile := range []string{Signature, Bundle} {
			sigPath := filepath.Join(tmpDir, sigFile)
			if _, statErr := os.Stat(sigPath); statErr == nil {
				err = os.WriteFile(sigPath, []byte("corrupted data"), 0o644)
				require.NoError(t, err)
			}
		}

		// Try to verify with corrupted signature(s)
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
	})

	t.Run("verification fails when zarf.yaml is modified after signing", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		// Create and sign package
		err := os.WriteFile(yamlPath, []byte("original content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Sign the package
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)

		// Modify the zarf.yaml after signing (tampering)
		err = os.WriteFile(yamlPath, []byte("modified content"), 0o644)
		require.NoError(t, err)

		// Verification should fail because content doesn't match signature
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
	})

	t.Run("verification falls back to legacy signature format", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		bundlePath := filepath.Join(tmpDir, Bundle)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		// Create and sign package
		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Sign the package
		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)
		require.FileExists(t, legacySignaturePath)

		// Remove the bundle if it exists to force legacy fallback
		err = os.Remove(bundlePath)
		require.NoError(t, err)

		// Verification should work with legacy signature (fallback path)
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.NoError(t, err, "verification should succeed with legacy signature format")
	})
}

// TestSignPackageBundleSignatureEnabled tests signing behavior when the BundleSignature
// feature flag is enabled. This test uses feature.Set() which is write-once, so it must
// be the last signing-related test to run. It is intentionally not parallel.
func TestSignPackageBundleSignatureEnabled(t *testing.T) {
	// Enable the BundleSignature feature flag via feature.Set()
	err := feature.Set([]feature.Feature{
		{Name: feature.BundleSignature, Enabled: true},
	})
	require.NoError(t, err)

	ctx := testutil.TestContext(t)

	t.Run("signing produces both bundle and legacy formats", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		bundlePath := filepath.Join(tmpDir, Bundle)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)
		require.FileExists(t, bundlePath, "bundle format signature should exist when feature is enabled")
		require.FileExists(t, legacySignaturePath, "legacy signature should also exist")
		require.NotNil(t, pkgLayout.Pkg.Build.Signed)
		require.True(t, *pkgLayout.Pkg.Build.Signed)
	})

	t.Run("version requirement persisted in zarf.yaml on disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		initialPkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Build: v1alpha1.ZarfBuildData{
				Architecture: "amd64",
			},
		}

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     initialPkg,
		}

		b, err := goyaml.Marshal(initialPkg)
		require.NoError(t, err)
		err = os.WriteFile(yamlPath, b, 0o644)
		require.NoError(t, err)

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)

		// Read the zarf.yaml from disk and verify version requirement
		updatedBytes, err := os.ReadFile(yamlPath)
		require.NoError(t, err)

		var updatedPkg v1alpha1.ZarfPackage
		err = goyaml.Unmarshal(updatedBytes, &updatedPkg)
		require.NoError(t, err)

		require.NotNil(t, updatedPkg.Build.Signed)
		require.True(t, *updatedPkg.Build.Signed)
	})

	t.Run("verification succeeds with bundle format", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		bundlePath := filepath.Join(tmpDir, Bundle)

		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)
		require.FileExists(t, bundlePath)

		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.NoError(t, err)
	})

	t.Run("verification falls back to legacy when bundle removed", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		bundlePath := filepath.Join(tmpDir, Bundle)
		legacySignaturePath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		signOpts := utils.DefaultSignBlobOptions()
		signOpts.KeyRef = "./testdata/cosign.key"
		signOpts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, signOpts)
		require.NoError(t, err)
		require.FileExists(t, bundlePath)
		require.FileExists(t, legacySignaturePath)

		// Remove bundle to force legacy fallback
		err = os.Remove(bundlePath)
		require.NoError(t, err)

		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.NoError(t, err, "should fall back to legacy signature")
	})
}

func TestGetDocumentation(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	// Helper function to set up a test package with documentation
	setupDocTest := func(t *testing.T, documentation map[string]string, tarFiles map[string]string) (*PackageLayout, string) {
		t.Helper()

		tmpDir := t.TempDir()
		pkgDir := filepath.Join(tmpDir, "package")
		require.NoError(t, os.MkdirAll(pkgDir, 0o700))

		// Create documentation.tar if tar files are provided
		if len(tarFiles) > 0 {
			docTempDir := t.TempDir()
			for filename, content := range tarFiles {
				require.NoError(t, os.WriteFile(filepath.Join(docTempDir, filename), []byte(content), 0o644))
			}

			tarPath := filepath.Join(pkgDir, DocumentationTar)
			err := createReproducibleTarballFromDir(docTempDir, "", tarPath, false)
			require.NoError(t, err)
		}

		pkg := v1alpha1.ZarfPackage{
			Metadata:      v1alpha1.ZarfMetadata{Name: "test"},
			Documentation: documentation,
		}

		pkgLayout := &PackageLayout{
			dirPath: pkgDir,
			Pkg:     pkg,
		}

		outputDir := filepath.Join(tmpDir, "output")
		return pkgLayout, outputDir
	}

	// Helper to assert file exists and has expected content
	assertFileContent := func(t *testing.T, path, expectedContent string) {
		t.Helper()
		require.FileExists(t, path)
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, expectedContent, string(content))
	}

	t.Run("extract all documentation files", func(t *testing.T) {
		pkgLayout, outputDir := setupDocTest(t,
			map[string]string{
				"readme":  "README.md",
				"license": "LICENSE",
			},
			map[string]string{
				"README.md": "readme content",
				"LICENSE":   "license content",
			},
		)

		err := pkgLayout.GetDocumentation(ctx, outputDir, nil)
		require.NoError(t, err)

		require.FileExists(t, filepath.Join(outputDir, "README.md"))
		require.FileExists(t, filepath.Join(outputDir, "LICENSE"))
	})

	t.Run("extract specific documentation keys", func(t *testing.T) {
		pkgLayout, outputDir := setupDocTest(t,
			map[string]string{
				"readme":  "README.md",
				"license": "LICENSE",
			},
			map[string]string{
				"README.md": "readme content",
				"LICENSE":   "license content",
			},
		)

		err := pkgLayout.GetDocumentation(ctx, outputDir, []string{"readme"})
		require.NoError(t, err)

		require.FileExists(t, filepath.Join(outputDir, "README.md"))
		require.NoFileExists(t, filepath.Join(outputDir, "LICENSE"))
	})

	t.Run("error when no documentation in package", func(t *testing.T) {
		pkgLayout, outputDir := setupDocTest(t,
			map[string]string{},
			nil,
		)

		err := pkgLayout.GetDocumentation(ctx, outputDir, nil)
		require.ErrorContains(t, err, "no documentation files found in package")
	})

	t.Run("error when key not found", func(t *testing.T) {
		pkgLayout, outputDir := setupDocTest(t,
			map[string]string{
				"readme": "README.md",
			},
			map[string]string{
				"README.md": "readme content",
			},
		)

		err := pkgLayout.GetDocumentation(ctx, outputDir, []string{"nonexistent"})
		require.ErrorContains(t, err, "not found in package documentation")
	})

	t.Run("extract single key when multiple files have same basename", func(t *testing.T) {
		pkgLayout, outputDir := setupDocTest(t,
			map[string]string{
				"readme1": "path/to/README.md",
				"readme2": "other/path/README.md",
			},
			map[string]string{
				"readme1-README.md": "readme1 content",
				"readme2-README.md": "readme2 content",
			},
		)

		err := pkgLayout.GetDocumentation(ctx, outputDir, []string{"readme1"})
		require.NoError(t, err)

		assertFileContent(t, filepath.Join(outputDir, "readme1-README.md"), "readme1 content")
	})
}

func TestLoadFromDir_VerificationStrategies(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	// Helper to create a test package directory with optional signature
	setupTestPackage := func(t *testing.T, signed bool) (string, string) {
		t.Helper()

		tmpDir := t.TempDir()
		pkgDir := filepath.Join(tmpDir, "package")
		require.NoError(t, os.MkdirAll(pkgDir, 0o700))

		// Create a minimal valid package
		pkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name:              "test-verification",
				Version:           "1.0.0",
				Architecture:      "amd64",
				AggregateChecksum: "placeholder",
			},
			Build: v1alpha1.ZarfBuildData{
				Architecture: "amd64",
			},
		}

		// Write zarf.yaml
		yamlPath := filepath.Join(pkgDir, ZarfYAML)
		yamlContent, err := goyaml.Marshal(pkg)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(yamlPath, yamlContent, 0o644))

		// Create a valid checksums file
		checksumsPath := filepath.Join(pkgDir, Checksums)
		// Calculate checksum of zarf.yaml
		checksumsContent := ""
		// Empty checksums with matching aggregate
		sha := sha256.Sum256([]byte(checksumsContent))
		pkg.Metadata.AggregateChecksum = hex.EncodeToString(sha[:])

		// Rewrite zarf.yaml with correct checksum
		yamlContent, err = goyaml.Marshal(pkg)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(yamlPath, yamlContent, 0o644))
		require.NoError(t, os.WriteFile(checksumsPath, []byte(checksumsContent), 0o644))

		if signed {
			// Sign the package
			pkgLayout := &PackageLayout{
				dirPath: pkgDir,
				Pkg:     pkg,
			}

			passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
				return []byte("test"), nil
			})
			signOpts := utils.DefaultSignBlobOptions()
			signOpts.KeyRef = "./testdata/cosign.key"
			signOpts.PassFunc = passFunc

			err = pkgLayout.SignPackage(ctx, signOpts)
			require.NoError(t, err)

			return pkgDir, "./testdata/cosign.pub"
		}

		return pkgDir, ""
	}

	t.Run("VerifyNever skips verification entirely", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, true) // Even with signed package

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyNever,
			PublicKeyPath:        "./testdata/cosign.pub",
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test-verification", pkgLayout.Pkg.Metadata.Name)
	})

	t.Run("VerifyNever with unsigned package succeeds", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, false)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyNever,
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
	})

	t.Run("VerifyIfPossible with signed package and valid key succeeds", func(t *testing.T) {
		pkgDir, pubKeyPath := setupTestPackage(t, true)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyIfPossible,
			PublicKeyPath:        pubKeyPath,
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test-verification", pkgLayout.Pkg.Metadata.Name)
	})

	t.Run("VerifyIfPossible with signed package and no key warns but continues", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, true)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyIfPossible,
			PublicKeyPath:        "", // No key provided
		}

		// Should warn but not fail
		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test-verification", pkgLayout.Pkg.Metadata.Name)
	})

	t.Run("VerifyIfPossible with signed package and wrong key warns but continues", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, true)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyIfPossible,
			PublicKeyPath:        "./testdata/nonexistent.pub",
		}

		// Should warn but not fail
		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
	})

	t.Run("VerifyIfPossible with unsigned package warns but continues", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, false)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyIfPossible,
			PublicKeyPath:        "./testdata/cosign.pub",
		}

		// Should warn about unsigned package but not fail
		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
	})

	t.Run("VerifyAlways with signed package and valid key succeeds", func(t *testing.T) {
		pkgDir, pubKeyPath := setupTestPackage(t, true)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyAlways,
			PublicKeyPath:        pubKeyPath,
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test-verification", pkgLayout.Pkg.Metadata.Name)
	})

	t.Run("VerifyAlways with signed package and invalid key fails", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, true)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyAlways,
			PublicKeyPath:        "./testdata/nonexistent.pub",
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.Error(t, err)
		require.Nil(t, pkgLayout)
		require.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("VerifyAlways with signed package and no key fails", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, true)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyAlways,
			PublicKeyPath:        "",
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.Error(t, err)
		require.Nil(t, pkgLayout)
		require.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("VerifyAlways with unsigned package fails", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, false)

		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyAlways,
			PublicKeyPath:        "./testdata/cosign.pub",
		}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.Error(t, err)
		require.Nil(t, pkgLayout)
		require.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("default strategy value is VerifyNever", func(t *testing.T) {
		pkgDir, _ := setupTestPackage(t, false)

		// Empty options - should default to VerifyNever (zero value)
		opts := PackageLayoutOptions{}

		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
	})
}

func TestLoadFromTar_VerificationStrategies(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	pathToPackage := filepath.Join("..", "testdata", "load-package", "compressed")
	tarPath := filepath.Join(pathToPackage, "zarf-package-test-amd64-0.0.1.tar.zst")

	t.Run("VerifyNever allows tarball load", func(t *testing.T) {
		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyNever,
		}

		pkgLayout, err := LoadFromTar(ctx, tarPath, opts)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, pkgLayout.Cleanup())
		})
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
	})

	t.Run("VerifyIfPossible warns but continues on unsigned tarball", func(t *testing.T) {
		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyIfPossible,
			PublicKeyPath:        "./testdata/cosign.pub",
		}

		// Should succeed with warning since package is unsigned
		pkgLayout, err := LoadFromTar(ctx, tarPath, opts)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, pkgLayout.Cleanup())
		})
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
	})

	t.Run("VerifyAlways fails on unsigned tarball", func(t *testing.T) {
		opts := PackageLayoutOptions{
			VerificationStrategy: VerifyAlways,
			PublicKeyPath:        "./testdata/cosign.pub",
		}

		pkgLayout, err := LoadFromTar(ctx, tarPath, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "signature verification failed")
		if pkgLayout != nil {
			t.Cleanup(func() {
				require.NoError(t, pkgLayout.Cleanup())
			})
		}
	})

	t.Run("default options work with tarball", func(t *testing.T) {
		// Verify zero-value options (VerifyNever) work correctly
		opts := PackageLayoutOptions{}

		pkgLayout, err := LoadFromTar(ctx, tarPath, opts)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, pkgLayout.Cleanup())
		})
		require.NotNil(t, pkgLayout)
		require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
	})
}

func TestValidatePackageIntegrity_SupplementalFiles(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	// Helper to create a package directory with an extra file and optional supplemental files list
	setupPackageWithExtra := func(t *testing.T, extraFileName string, supplementalFiles []string) string {
		t.Helper()

		tmpDir := t.TempDir()
		pkgDir := filepath.Join(tmpDir, "package")
		require.NoError(t, os.MkdirAll(pkgDir, 0o700))

		isSigned := true

		pkg := v1alpha1.ZarfPackage{
			Kind: v1alpha1.ZarfPackageConfig,
			Metadata: v1alpha1.ZarfMetadata{
				Name:              "test-supplemental",
				Version:           "1.0.0",
				AggregateChecksum: "placeholder",
			},
			Build: v1alpha1.ZarfBuildData{
				Architecture:      "amd64",
				Signed:            &isSigned,
				SupplementalFiles: supplementalFiles,
			},
		}

		checksumsContent := ""
		checksumsHash := sha256.Sum256([]byte(checksumsContent))
		pkg.Metadata.AggregateChecksum = hex.EncodeToString(checksumsHash[:])

		yamlContent, err := goyaml.Marshal(pkg)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ZarfYAML), yamlContent, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, Checksums), []byte(checksumsContent), 0o644))

		if extraFileName != "" {
			require.NoError(t, os.WriteFile(filepath.Join(pkgDir, extraFileName), []byte("extra"), 0o644))
		}

		return pkgDir
	}

	t.Run("supplemental file is excluded from integrity check", func(t *testing.T) {
		pkgDir := setupPackageWithExtra(t, "zarf.future.sig", []string{Checksums, "zarf.future.sig"})

		opts := PackageLayoutOptions{VerificationStrategy: VerifyNever}
		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
	})

	t.Run("unknown file without supplemental listing fails integrity check", func(t *testing.T) {
		pkgDir := setupPackageWithExtra(t, "injected.bin", []string{Checksums})

		opts := PackageLayoutOptions{VerificationStrategy: VerifyNever}
		_, err := LoadFromDir(ctx, pkgDir, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "additional files not present in the checksum")
	})

	t.Run("backward compat: old package without supplemental files field loads fine", func(t *testing.T) {
		// Simulates an old package with no SupplementalFiles set but with
		// a legacy signature file present (covered by hardcoded exclusions)
		pkgDir := setupPackageWithExtra(t, Signature, nil)

		opts := PackageLayoutOptions{VerificationStrategy: VerifyNever}
		pkgLayout, err := LoadFromDir(ctx, pkgDir, opts)
		require.NoError(t, err)
		require.NotNil(t, pkgLayout)
	})
}

func TestSignPackage_PopulatesSupplementalFiles(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	t.Run("signing populates supplemental files with checksums and signature", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					SupplementalFiles: []string{Checksums},
				},
			},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)

		require.Contains(t, pkgLayout.Pkg.Build.SupplementalFiles, Checksums)
		require.Contains(t, pkgLayout.Pkg.Build.SupplementalFiles, Signature)
	})

	t.Run("signing rollback restores original supplemental files on failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		original := []string{Checksums}
		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg: v1alpha1.ZarfPackage{
				Build: v1alpha1.ZarfBuildData{
					SupplementalFiles: original,
				},
			},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("wrongpassword"), nil
		})
		opts := utils.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.Error(t, err)
		require.Equal(t, []string{Checksums}, pkgLayout.Pkg.Build.SupplementalFiles)
	})
}
