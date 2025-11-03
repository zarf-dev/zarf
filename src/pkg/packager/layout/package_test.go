// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	zarfCosign "github.com/zarf-dev/zarf/src/internal/cosign"
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
		signedPath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := zarfCosign.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)
		require.FileExists(t, signedPath)
		require.NotNil(t, pkgLayout.Pkg.Build.Signed)
		require.True(t, *pkgLayout.Pkg.Build.Signed)
	})

	t.Run("wrong password", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		signedPath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("wrongpassword"), nil
		})
		opts := zarfCosign.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.ErrorContains(t, err, "failed to sign package")
		require.ErrorContains(t, err, "reading key: decrypt: encrypted: decryption failed")
		require.NoFileExists(t, signedPath)
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
		opts := zarfCosign.DefaultSignBlobOptions()
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
		opts := zarfCosign.DefaultSignBlobOptions()
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
		opts := zarfCosign.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err := pkgLayout.SignPackage(ctx, opts)
		require.EqualError(t, err, "invalid package layout: dirPath is empty")
	})

	t.Run("overwrite existing signature", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		signedPath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		// Create an existing signature file
		err = os.WriteFile(signedPath, []byte("old signature"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		passFunc := cosign.PassFunc(func(_ bool) ([]byte, error) {
			return []byte("test"), nil
		})
		opts := zarfCosign.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		// Should overwrite the existing signature (with warning logged)
		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)
		require.FileExists(t, signedPath)

		// Verify the signature was overwritten (not the old content)
		content, err := os.ReadFile(signedPath)
		require.NoError(t, err)
		require.NotEqual(t, "old signature", string(content))
	})

	t.Run("skip signing when ShouldSign returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		signedPath := filepath.Join(tmpDir, Signature)

		err := os.WriteFile(yamlPath, []byte("foobar"), 0o644)
		require.NoError(t, err)

		pkgLayout := &PackageLayout{
			dirPath: tmpDir,
			Pkg:     v1alpha1.ZarfPackage{},
		}

		// Empty options - no signing key material configured
		opts := zarfCosign.SignBlobOptions{}

		// Should skip signing without error
		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)
		require.NoFileExists(t, signedPath)
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
		opts := zarfCosign.DefaultSignBlobOptions()
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
		opts := zarfCosign.DefaultSignBlobOptions()
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
		opts := zarfCosign.DefaultSignBlobOptions()
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
		opts := zarfCosign.SignBlobOptions{}

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
		opts := zarfCosign.DefaultSignBlobOptions()
		opts.KeyRef = "./testdata/cosign.key"
		opts.PassFunc = passFunc

		err = pkgLayout.SignPackage(ctx, opts)
		require.NoError(t, err)

		// Verify signature file exists
		signaturePath := filepath.Join(tmpDir, Signature)
		require.FileExists(t, signaturePath)

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
		setupFunc      func(t *testing.T) (*PackageLayout, zarfCosign.SignBlobOptions)
		expectedErr    string
		expectSigned   bool
		expectSignFile bool
	}{
		{
			name: "package with existing false Signed value gets updated on success",
			setupFunc: func(t *testing.T) (*PackageLayout, zarfCosign.SignBlobOptions) {
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
				opts := zarfCosign.DefaultSignBlobOptions()
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
			setupFunc: func(t *testing.T) (*PackageLayout, zarfCosign.SignBlobOptions) {
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
				opts := zarfCosign.DefaultSignBlobOptions()
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
			setupFunc: func(t *testing.T) (*PackageLayout, zarfCosign.SignBlobOptions) {
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
				opts := zarfCosign.DefaultSignBlobOptions()
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
			setupFunc: func(t *testing.T) (*PackageLayout, zarfCosign.SignBlobOptions) {
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
				opts := zarfCosign.DefaultSignBlobOptions()
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
			setupFunc: func(t *testing.T) (*PackageLayout, zarfCosign.SignBlobOptions) {
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
				opts := zarfCosign.DefaultSignBlobOptions()
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
		signedPath := filepath.Join(tmpDir, Signature)

		// Create and sign a package
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
		require.FileExists(t, signedPath)

		// Verify the signature
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
		require.Contains(t, err.Error(), "signature not found")
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
		signedPath := filepath.Join(tmpDir, Signature)

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
		require.FileExists(t, signedPath)

		// Try to verify without providing a key
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "" // Empty key

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.EqualError(t, err, "package is signed but no key was provided")
	})

	t.Run("verification fails when signature is corrupted", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		signedPath := filepath.Join(tmpDir, Signature)

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
		require.FileExists(t, signedPath)

		// Corrupt the signature
		err = os.WriteFile(signedPath, []byte("corrupted signature data"), 0o644)
		require.NoError(t, err)

		// Try to verify with corrupted signature
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
	})

	t.Run("verification fails when zarf.yaml is modified after signing", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, ZarfYAML)
		signedPath := filepath.Join(tmpDir, Signature)

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
		require.FileExists(t, signedPath)

		// Modify the zarf.yaml after signing (tampering)
		err = os.WriteFile(yamlPath, []byte("modified content"), 0o644)
		require.NoError(t, err)

		// Verification should fail because content doesn't match signature
		verifyOpts := utils.DefaultVerifyBlobOptions()
		verifyOpts.KeyRef = "./testdata/cosign.pub"

		err = pkgLayout.VerifyPackageSignature(ctx, verifyOpts)
		require.Error(t, err)
	})
}
