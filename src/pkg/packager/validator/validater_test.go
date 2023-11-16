// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validator contains functions for verifying zarf yaml files are valid
package validator

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
		t.Errorf("error unmarshalling yaml %v", err)
	}
	return unmarshalledYaml
}

func TestValidateSchema(t *testing.T) {
	getZarfSchema := func(t *testing.T) []byte {
		t.Helper()
		file, err := os.ReadFile("../../../../zarf.schema.json")
		if err != nil {
			t.Errorf("error reading file: %s", err)
		}
		return file
	}

	t.Run("validate schema success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[interface{}](t, goodZarfPackage)
		zarfSchema := getZarfSchema(t)
		err := validateSchema(unmarshalledYaml, zarfSchema)
		require.NoError(t, err)
	})

	t.Run("validate schema fail", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[interface{}](t, badZarfPackage)
		zarfSchema := getZarfSchema(t)
		err := validateSchema(unmarshalledYaml, zarfSchema)
		errorMessage := zarfInvalidPrefix + `
 - components.0.import: Additional property not-path is not allowed
 - components.1.import.path: Invalid type. Expected: string, given: integer`
		require.EqualError(t, err, errorMessage)
	})

	t.Run("Template in component import success", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, goodZarfPackage)
		err := checkForVarInComponentImport(unmarshalledYaml)
		require.NoError(t, err)
	})

	t.Run("Template in component import failure", func(t *testing.T) {
		unmarshalledYaml := readAndUnmarshalYaml[types.ZarfPackage](t, badZarfPackage)
		err := checkForVarInComponentImport(unmarshalledYaml)
		errorMessage := zarfWarningPrefix + " component.2.import.path will not resolve ZARF_PKG_TMPL_* variables, " +
			"component.3.import.url will not resolve ZARF_PKG_TMPL_* variables"
		require.EqualError(t, err, errorMessage)
	})
}
