// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
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
  - busybox@sha256:3fbc632167424a6d997e74f52b878d7cc478225cffac6bc977eedfe51c7f4e79
  - busybox:latest
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
		require.EqualError(t, validator.errors[0], ".components.[0].import: Additional property not-path is not allowed")
		require.EqualError(t, validator.errors[1], ".components.[1].import.path: Invalid type. Expected: string, given: integer")
	})

	t.Run("Template in component import success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, goodZarfPackage)
		validator := Validator{typedZarfPackage: unmarshalledYaml}
		checkForVarInComponentImport(&validator)
		require.Empty(t, validator.warnings)
	})

	t.Run("Template in component import failure", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, badZarfPackage)
		validator := Validator{typedZarfPackage: unmarshalledYaml}
		checkForVarInComponentImport(&validator)
		require.Equal(t, validator.warnings[0], ".components.[2].import.path: Will not resolve ZARF_PKG_TMPL_* variables")
		require.Equal(t, validator.warnings[1], ".components.[3].import.url: Will not resolve ZARF_PKG_TMPL_* variables")
	})

	t.Run("Unpinnned repo warning", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, badZarfPackage)
		validator := Validator{typedZarfPackage: unmarshalledYaml}
		checkforUnpinnedRepos(&validator)
		require.Equal(t, validator.warnings[0], ".components.[4].repos.[0]: Unpinned repository")
		require.Equal(t, len(validator.warnings), 1)
	})

	t.Run("Unpinnned image warning", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, badZarfPackage)
		validator := Validator{typedZarfPackage: unmarshalledYaml}
		checkForUnpinnedImages(&validator)
		require.Equal(t, validator.warnings[0], ".components.[4].images.[3]: Unpinned image")
		require.Equal(t, len(validator.warnings), 1)
	})

	t.Run("Unpinnned file warning", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, badZarfPackage)
		validator := Validator{typedZarfPackage: unmarshalledYaml}
		checkForUnpinnedFiles(&validator)
		require.Equal(t, validator.warnings[0], ".components.[4].files.[1]: Unpinned file")
		require.Equal(t, len(validator.warnings), 1)
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

	t.Run("image is pinned", func(t *testing.T) {
		input := "ghcr.io/defenseunicorns/pepr/controller:v0.15.0"
		expcected := true
		acutal := imageIsPinned(input)
		require.Equal(t, expcected, acutal)
	})

	t.Run("image is unpinned", func(t *testing.T) {
		input := "ghcr.io/defenseunicorns/pepr/controller"
		expcected := false
		acutal := imageIsPinned(input)
		require.Equal(t, expcected, acutal)
	})

	t.Run("image is pinned and has port", func(t *testing.T) {
		input := "registry.com:8080/defenseunicorns/whatever"
		expcected := false
		acutal := imageIsPinned(input)
		require.Equal(t, expcected, acutal)
	})
	//Image signature ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig
}
