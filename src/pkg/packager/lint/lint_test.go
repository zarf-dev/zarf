// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

const badZarfPackage = `
kind: ZarfInitConfig
metadata:
  name: init
  description: Testing bad yaml

components:
- name: first-test-component
  import:
    not-path: packages/distros/k3s
- name: import-test
  import:
    path: 123123

- name: import-test
  import:
    path: "###ZARF_PKG_TMPL_ZEBRA###"

- name: import-url
  import:
    url: "oci://###ZARF_PKG_TMPL_ZEBRA###"

- name: full-repo
  repos:
  - https://github.com/defenseunicorns/zarf-public-test.git
  - https://dev.azure.com/defenseunicorns/zarf-public-test/_git/zarf-public-test@v0.0.1
  - https://gitlab.com/gitlab-org/build/omnibus-mirror/pcre2/-/tree/vreverse?ref_type=heads
  images:
  - ghcr.io/kiwix/kiwix-serve:3.5.0-2
  - registry.com:9001/whatever/image:1.0.0
  - busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79
  - busybox:latest
  - badimage:badimage@@sha256:3fbc632167424a6d997e74f5
  files:
  - source: https://github.com/k3s-io/k3s/releases/download/v1.28.2+k3s1/k3s
    shasum: 2f041d37a2c6d54d53e106e1c7713bc48f806f3919b0d9e092f5fcbdc55b41cf
  - source: file-without-shasum.txt
`

const goodZarfPackage = `
kind: ZarfPackageConfig
metadata:
  name: good-zarf-package

components:
  - name: baseline
    required: true
`

func readAndUnmarshalYaml[T interface{}](t *testing.T, yamlString string) T {
	t.Helper()
	var unmarshalledYaml T
	err := goyaml.Unmarshal([]byte(yamlString), &unmarshalledYaml)
	if err != nil {
		t.Errorf("error unmarshalling yaml: %v", err)
	}
	return unmarshalledYaml
}

func TestValidateSchema(t *testing.T) {
	getZarfSchema := func(t *testing.T) []byte {
		t.Helper()
		file, err := os.ReadFile("../../../../zarf.schema.json")
		if err != nil {
			t.Errorf("error reading file: %v", err)
		}
		return file
	}

	t.Run("validate schema success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[interface{}](t, goodZarfPackage)
		validator := Validator{untypedZarfPackage: unmarshalledYaml, jsonSchema: getZarfSchema(t)}
		err := validateSchema(&validator)
		require.NoError(t, err)
		require.Empty(t, validator.errors)
	})

	t.Run("validate schema fail", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[interface{}](t, badZarfPackage)
		validator := Validator{untypedZarfPackage: unmarshalledYaml, jsonSchema: getZarfSchema(t)}
		err := validateSchema(&validator)
		require.NoError(t, err)
		require.Equal(t, validator.errors[0].String(), ".components.[0].import: Additional property not-path is not allowed")
		require.Equal(t, validator.errors[1].String(), ".components.[1].import.path: Invalid type. Expected: string, given: integer")
	})

	t.Run("Template in component import success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, goodZarfPackage)
		validator := Validator{typedZarfPackage: unmarshalledYaml}
		lintComponents(&validator)
		require.Empty(t, validator.warnings)
		require.Empty(t, validator.errors)
	})

	t.Run("Template in component import failure", func(t *testing.T) {
		validator := Validator{}
		pathComponent := types.ZarfComponent{Import: types.ZarfComponentImport{Path: "###ZARF_PKG_TMPL_ZEBRA###"}}
		URLComponent := types.ZarfComponent{Import: types.ZarfComponentImport{URL: "oci://###ZARF_PKG_TMPL_ZEBRA###"}}
		checkForVarInComponentImport(&validator, 2, pathComponent, "")
		checkForVarInComponentImport(&validator, 3, URLComponent, "")
		require.Equal(t,
			".components.[2].import.path: Zarf does not evaluate variables at component.x.import.path ###ZARF_PKG_TMPL_ZEBRA###",
			validator.warnings[0].String())
		require.Equal(t,
			".components.[3].import.url: Zarf does not evaluate variables at component.x.import.url oci://###ZARF_PKG_TMPL_ZEBRA###",
			validator.warnings[1].String())
	})

	t.Run("Unpinnned repo warning", func(t *testing.T) {
		validator := Validator{}
		unpinnedRepo := "https://github.com/defenseunicorns/zarf-public-test.git"
		component := types.ZarfComponent{Repos: []string{
			unpinnedRepo,
			"https://dev.azure.com/defenseunicorns/zarf-public-test/_git/zarf-public-test@v0.0.1"}}
		checkForUnpinnedRepos(&validator, 0, component, "")
		require.Equal(t,
			fmt.Sprintf(".components.[0].repos.[0]: Unpinned repository %s", unpinnedRepo),
			validator.warnings[0].String())
		require.Equal(t, len(validator.warnings), 1)
	})

	t.Run("Unpinnned image warning", func(t *testing.T) {
		validator := Validator{}
		unpinnedImage := "registry.com:9001/whatever/image:1.0.0"
		badImage := "badimage:badimage@@sha256:3fbc632167424a6d997e74f5"
		component := types.ZarfComponent{Images: []string{
			unpinnedImage,
			"busybox:latest@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79",
			badImage}}
		checkForUnpinnedImages(&validator, 0, component, "")
		require.Equal(t, fmt.Sprintf(".components.[0].images.[0]: Unpinned image %s", unpinnedImage), validator.warnings[0].String())
		require.Equal(t, len(validator.warnings), 1)
		expectedErr := fmt.Sprintf(".components.[0].images.[2]: Invalid image format %s", badImage)
		require.Equal(t, validator.errors[0].String(), expectedErr)
		require.Equal(t, len(validator.errors), 1)
	})

	t.Run("Unpinnned file warning", func(t *testing.T) {
		validator := Validator{}
		filename := "http://example.com/file.zip"
		zarfFiles := []types.ZarfFile{
			{
				Source: filename,
			},
		}
		component := types.ZarfComponent{Files: zarfFiles}
		checkForUnpinnedFiles(&validator, 0, component, "")
		expected := fmt.Sprintf(".components.[0].files.[0]: Unpinned file %s", filename)
		require.Equal(t, expected, validator.warnings[0].String())
		require.Equal(t, 1, len(validator.warnings))
	})

	t.Run("Wrap standalone numbers in bracket", func(t *testing.T) {
		input := "components12.12.import.path"
		expected := ".components12.[12].import.path"
		acutal := makeFieldPathYqCompat(input)
		require.Equal(t, expected, acutal)
	})

	t.Run("root doesn't change", func(t *testing.T) {
		input := "(root)"
		acutal := makeFieldPathYqCompat(input)
		require.Equal(t, input, acutal)
	})

	t.Run("isImagePinned", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			input    string
			expected bool
			err      error
		}{
			{
				input:    "registry.com:8080/defenseunicorns/whatever",
				expected: false,
				err:      nil,
			},
			{
				input:    "ghcr.io/defenseunicorns/pepr/controller:v0.15.0",
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
				acutal, err := isPinnedImage(tc.input)
				if err != nil {
					require.EqualError(t, err, tc.err.Error())
				}
				require.Equal(t, tc.expected, acutal)
			})
		}
	})
}
