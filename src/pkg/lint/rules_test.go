// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestValidateComponent(t *testing.T) {
	t.Parallel()

	t.Run("Unpinnned repo warning", func(t *testing.T) {
		t.Parallel()
		unpinnedRepo := "https://github.com/zarf-dev/zarf-public-test.git"
		component := types.ZarfComponent{Repos: []string{
			unpinnedRepo,
			"https://dev.azure.com/zarf-dev/zarf-public-test/_git/zarf-public-test@v0.0.1",
		}}
		findings := checkForUnpinnedRepos(component, 0)
		expected := []PackageFinding{
			{
				Item:        unpinnedRepo,
				Description: "Unpinned repository",
				Severity:    SevWarn,
				YqPath:      ".components.[0].repos.[0]",
			},
		}
		require.Equal(t, expected, findings)
	})

	t.Run("Unpinnned image warning", func(t *testing.T) {
		t.Parallel()
		unpinnedImage := "registry.com:9001/whatever/image:1.0.0"
		badImage := "badimage:badimage@@sha256:3fbc632167424a6d997e74f5"
		cosignSignature := "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig"
		cosignAttestation := "ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.att"
		component := types.ZarfComponent{Images: []string{
			unpinnedImage,
			"busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
			badImage,
			cosignSignature,
			cosignAttestation,
		}}
		findings := checkForUnpinnedImages(component, 0)
		expected := []PackageFinding{
			{
				Item:        unpinnedImage,
				Description: "Image not pinned with digest",
				Severity:    SevWarn,
				YqPath:      ".components.[0].images.[0]",
			},
			{
				Item:        badImage,
				Description: "Failed to parse image reference",
				Severity:    SevWarn,
				YqPath:      ".components.[0].images.[2]",
			},
		}
		require.Equal(t, expected, findings)
	})

	t.Run("Unpinnned file warning", func(t *testing.T) {
		t.Parallel()
		fileURL := "http://example.com/file.zip"
		localFile := "local.txt"
		zarfFiles := []types.ZarfFile{
			{
				Source: fileURL,
			},
			{
				Source: localFile,
			},
			{
				Source: fileURL,
				Shasum: "fake-shasum",
			},
		}
		component := types.ZarfComponent{Files: zarfFiles}
		findings := checkForUnpinnedFiles(component, 0)
		expected := []PackageFinding{
			{
				Item:        fileURL,
				Description: "No shasum for remote file",
				Severity:    SevWarn,
				YqPath:      ".components.[0].files.[0]",
			},
		}
		require.Equal(t, expected, findings)
		require.Len(t, findings, 1)
	})

	t.Run("isImagePinned", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			input    string
			expected bool
			err      error
		}{
			{
				input:    "registry.com:8080/zarf-dev/whatever",
				expected: false,
				err:      nil,
			},
			{
				input:    "ghcr.io/zarf-dev/pepr/controller:v0.15.0",
				expected: false,
				err:      nil,
			},
			{
				input:    "busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
				expected: true,
				err:      nil,
			},
			{
				input:    "busybox:bad/image",
				expected: false,
				err:      errors.New("invalid reference format"),
			},
			{
				input:    "busybox:###ZARF_PKG_TMPL_BUSYBOX_IMAGE###",
				expected: true,
				err:      nil,
			},
		}
		for _, tc := range tests {
			t.Run(tc.input, func(t *testing.T) {
				actual, err := isPinnedImage(tc.input)
				if err != nil {
					require.EqualError(t, err, tc.err.Error())
				}
				require.Equal(t, tc.expected, actual)
			})
		}
	})
}
