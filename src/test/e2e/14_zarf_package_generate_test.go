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
		schema, _, err := value.LoadValidatedSchema(packagePath, schemaPath)
		require.NoError(t, err)
		require.NotNil(t, schema)

		props, ok := schema["properties"].(map[string]any)
		require.True(t, ok)

		app, ok := props["app"].(map[string]any)
		require.True(t, ok)
		appProps, ok := app["properties"].(map[string]any)
		require.True(t, ok)

		aName, ok := appProps["name"].(map[string]any)
		require.True(t, ok)
		// .app.name should take the type 'string' from the parent values.yaml
		require.Equal(t, "string", aName["type"])
		// .app.name should take the description from the parent values.schema.json
		require.Equal(t, "Application name", aName["description"])

		aReplicas, ok := appProps["replicas"].(map[string]any)
		require.True(t, ok)
		// .app.replicas should take the type 'number' from the parent values.yaml
		require.Equal(t, "number", aReplicas["type"])
		// .app.replicas should take the description from the parent values.schema.json
		require.Equal(t, "Replica count", aReplicas["description"])

		backend, ok := props["backend"].(map[string]any)
		require.True(t, ok)
		backendProps, ok := backend["properties"].(map[string]any)
		require.True(t, ok)

		bName, ok := backendProps["name"].(map[string]any)
		require.True(t, ok)
		// .backend.name should take the type 'string' from the child values.yaml
		require.Equal(t, "string", bName["type"])
		// .backend.name should take the description from the child values.schema.json
		require.Equal(t, "Backend name", bName["description"])

		bReplicas, ok := backendProps["replicaCount"].(map[string]any)
		require.True(t, ok)
		// .backend.replicas should take the type 'number' from the child values.yaml
		require.Equal(t, "number", bReplicas["type"])
		// .backend.replicas should take the description from the child values.schema.json
		require.Equal(t, "Replica count", bReplicas["description"])

		// .backend.service.port should be pulled in from the child's mapped chart
		bService, ok := backendProps["service"].(map[string]any)
		require.True(t, ok)
		bServiceProps, ok := bService["properties"].(map[string]any)
		require.True(t, ok)
		bPort, ok := bServiceProps["port"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "number", bPort["type"])

		// .network should be pulled in from the parent's mapped chart
		network, ok := props["network"].(map[string]any)
		require.True(t, ok)
		networkProps, ok := network["properties"].(map[string]any)
		require.True(t, ok)
		port, ok := networkProps["port"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "number", port["type"])

		// .oldField should be dropped from the values.schema.json
		_, hasOldField := props["oldField"]
		require.False(t, hasOldField)
	})
}
