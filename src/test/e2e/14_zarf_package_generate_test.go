// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

func TestZarfDevGenerate(t *testing.T) {
	t.Log("E2E: Zarf Dev Generate")

	t.Run("Test generate podinfo", func(t *testing.T) {
		tmpDir := t.TempDir()

		url := "https://github.com/stefanprodan/podinfo.git"
		version := "6.4.0"
		gitPath := "charts/podinfo"

		stdOut, stdErr, err := e2e.Zarf(t, "dev", "generate", "podinfo", "--url", url, "--version", version, "--gitPath", gitPath, "--output-directory", tmpDir)
		require.NoError(t, err, stdOut, stdErr)

		zarfPackage := v1alpha1.ZarfPackage{}
		packageLocation := filepath.Join(tmpDir, layout.ZarfYAML)
		err = utils.ReadYaml(packageLocation, &zarfPackage)
		require.NoError(t, err)
		require.Equal(t, zarfPackage.Components[0].Charts[0].URL, url)
		require.Equal(t, zarfPackage.Components[0].Charts[0].Version, version)
		require.Equal(t, zarfPackage.Components[0].Charts[0].GitPath, gitPath)
		require.NotEmpty(t, zarfPackage.Components[0].Images)
	})

	t.Run("Test generate-schema merges inferred data into existing schema", func(t *testing.T) {
		packagePath := t.TempDir()
		err := helpers.CreatePathAndCopy("src/test/packages/14-generate-schema", packagePath)
		require.NoError(t, err)

		stdOut, stdErr, err := e2e.ZarfInDir(t, packagePath, "dev", "generate-schema", ".", "--features=values=true")
		require.NoError(t, err, stdOut, stdErr)

		schemaPath := filepath.Join(packagePath, "values.schema.json")
		schema, err := value.LoadJSONSchema(schemaPath)
		require.NoError(t, err)
		require.NotNil(t, schema)

		props, ok := schema["properties"].(map[string]any)
		require.True(t, ok)

		app, ok := props["app"].(map[string]any)
		require.True(t, ok)
		appProps, ok := app["properties"].(map[string]any)
		require.True(t, ok)

		name, ok := appProps["name"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "string", name["type"])
		require.Equal(t, "Application name", name["description"])

		replicas, ok := appProps["replicas"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "number", replicas["type"])
		require.Equal(t, "Replica count", replicas["description"])

		network, ok := props["network"].(map[string]any)
		require.True(t, ok)
		networkProps, ok := network["properties"].(map[string]any)
		require.True(t, ok)
		port, ok := networkProps["port"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "number", port["type"])

		_, hasOldField := props["oldField"]
		require.False(t, hasOldField)
	})
}
