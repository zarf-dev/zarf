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
		require.EqualError(t, validator.errors[0], "components.0.import: Additional property not-path is not allowed")
		require.EqualError(t, validator.errors[1], "components.1.import.path: Invalid type. Expected: string, given: integer")
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
		require.Equal(t, validator.warnings[0], "component.2.import.path will not resolve ZARF_PKG_TMPL_* variables")
		require.Equal(t, validator.warnings[1], "component.3.import.url will not resolve ZARF_PKG_TMPL_* variables")
	})

	t.Run("Validator Error formatting", func(t *testing.T) {
		error1 := errors.New("components.0.import: Additional property not-path is not allowed")
		error2 := errors.New("components.1.import.path: Invalid type. Expected: string, given: integer")
		validator := Validator{errors: []error{error1, error2}}
		errorMessage := fmt.Sprintf("%s\n - %s\n - %s", validatorInvalidPrefix, error1.Error(), error2.Error())
		require.EqualError(t, validator.getFormatedError(), errorMessage)
	})

	t.Run("Validator Warning formatting", func(t *testing.T) {
		warning1 := "components.0.import: Additional property not-path is not allowed"
		warning2 := "components.1.import.path: Invalid type. Expected: string, given: integer"
		validator := Validator{warnings: []string{warning1, warning2}}
		message := fmt.Sprintf("%s %s, %s", validatorWarningPrefix, warning1, warning2)
		require.Equal(t, validator.getFormatedWarning(), message)
	})
}
