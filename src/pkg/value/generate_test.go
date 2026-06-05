// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateJSONSchema(t *testing.T) {
	t.Run("infers nested types", func(t *testing.T) {
		vals := Values{
			"name":     "zarf",
			"replicas": uint64(3),
			"enabled":  true,
			"ports":    []any{uint64(80)},
			"image": map[string]any{
				"tag": "v1.2.3",
			},
		}

		schema := GenerateJSONSchema(vals)

		require.Equal(t, "http://json-schema.org/draft-07/schema#", schema["$schema"])
		require.Equal(t, "object", schema["type"])

		props, ok := schema["properties"].(map[string]any)
		require.True(t, ok)

		name, ok := props["name"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "string", name["type"])

		replicas, ok := props["replicas"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "number", replicas["type"])

		enabled, ok := props["enabled"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "boolean", enabled["type"])

		ports, ok := props["ports"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "array", ports["type"])
		items, ok := ports["items"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "number", items["type"])

		image, ok := props["image"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "object", image["type"])
		imageProps, ok := image["properties"].(map[string]any)
		require.True(t, ok)
		tag, ok := imageProps["tag"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "string", tag["type"])
	})
}

func TestReconcileJSONSchema(t *testing.T) {
	t.Run("updates structure and preserves handcrafted metadata", func(t *testing.T) {
		existing := map[string]any{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type":    "object",
			"properties": map[string]any{
				"site": map[string]any{
					"type":        "object",
					"description": "Site configuration",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Site name",
							"minLength":   float64(1),
						},
						"legacy": map[string]any{
							"type":        "string",
							"description": "Old field",
						},
					},
					"required": []any{"name"},
				},
				"features": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":      "string",
						"enum":      []any{"alpha", "beta"},
						"minLength": float64(2),
					},
				},
				"oldField": map[string]any{
					"type": "string",
				},
			},
		}

		inferred := map[string]any{
			"$schema": "http://json-schema.org/draft-07/schema#",
			"type":    "object",
			"properties": map[string]any{
				"site": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
				"features": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "number",
					},
				},
				"newField": map[string]any{
					"type": "string",
				},
			},
		}

		result := ReconcileJSONSchema(existing, inferred)

		props, ok := result["properties"].(map[string]any)
		require.True(t, ok)
		site, ok := props["site"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Site configuration", site["description"]) // preserved
		assert.Equal(t, []any{"name"}, site["required"])           // preserved

		siteProps, ok := site["properties"].(map[string]any)
		require.True(t, ok)
		_, hasLegacy := siteProps["legacy"]
		assert.False(t, hasLegacy) // removed (not inferred)

		name, ok := siteProps["name"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Site name", name["description"])   // preserved
		assert.InDelta(t, float64(1), name["minLength"], 0) // preserved
		assert.Equal(t, "string", name["type"])             // structural sync

		features, ok := props["features"].(map[string]any)
		require.True(t, ok)
		items, ok := features["items"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "number", items["type"])               // updated from inferred
		assert.Equal(t, []any{"alpha", "beta"}, items["enum"]) // preserved
		assert.InDelta(t, float64(2), items["minLength"], 0)   // preserved

		_, hasNewField := props["newField"]
		assert.True(t, hasNewField) // added (inferred)

		_, hasOldField := props["oldField"]
		assert.False(t, hasOldField) // removed (not contained in inferred)
	})
}
