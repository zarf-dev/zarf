// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckNoExternalRefs(t *testing.T) {
	t.Run("schema with no refs passes", func(t *testing.T) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		}
		require.NoError(t, CheckNoExternalRefs(schema))
	})

	t.Run("internal fragment ref passes", func(t *testing.T) {
		schema := map[string]any{
			"definitions": map[string]any{
				"Name": map[string]any{"type": "string"},
			},
			"properties": map[string]any{
				"name": map[string]any{"$ref": "#/definitions/Name"},
			},
		}
		require.NoError(t, CheckNoExternalRefs(schema))
	})

	t.Run("external file ref is rejected", func(t *testing.T) {
		schema := map[string]any{"$ref": "./defs/name.json"}
		err := CheckNoExternalRefs(schema)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "$ref")
		assert.Contains(t, err.Error(), "./defs/name.json")
	})

	t.Run("nested external ref is rejected", func(t *testing.T) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"$ref": "./defs/name.json"},
			},
		}
		err := CheckNoExternalRefs(schema)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "$ref")
	})

	t.Run("external ref inside allOf slice is rejected", func(t *testing.T) {
		schema := map[string]any{
			"allOf": []any{
				map[string]any{"$ref": "./base.json"},
			},
		}
		err := CheckNoExternalRefs(schema)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "$ref")
	})

	t.Run("HTTP URI ref is rejected", func(t *testing.T) {
		schema := map[string]any{"$ref": "https://example.com/schemas/base.json"}
		err := CheckNoExternalRefs(schema)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "$ref")
	})
}

func TestMergeSchemas(t *testing.T) {
	t.Run("child-only property is inherited", func(t *testing.T) {
		parent := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
		child := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"extra": map[string]any{"type": "string"},
			},
		}
		result := MergeSchemas(parent, child)
		props, ok := result["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		assert.Contains(t, props, "extra")
	})

	t.Run("parent wins on same property", func(t *testing.T) {
		parent := map[string]any{
			"properties": map[string]any{
				"image": map[string]any{"type": "string", "description": "parent description"},
			},
		}
		child := map[string]any{
			"properties": map[string]any{
				"image": map[string]any{"type": "string", "description": "child description"},
			},
		}
		result := MergeSchemas(parent, child)
		props, ok := result["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		image, ok := props["image"].(map[string]any)
		require.True(t, ok, "image should be a map")
		assert.Equal(t, "parent description", image["description"])
	})

	t.Run("required is union of parent and child", func(t *testing.T) {
		parent := map[string]any{"required": []any{"tag"}}
		child := map[string]any{"required": []any{"registry"}}
		result := MergeSchemas(parent, child)
		req, ok := result["required"].([]any)
		require.True(t, ok, "required should be a slice")
		assert.ElementsMatch(t, []any{"tag", "registry"}, req)
	})

	t.Run("required deduplicates overlapping entries", func(t *testing.T) {
		parent := map[string]any{"required": []any{"name", "tag"}}
		child := map[string]any{"required": []any{"name", "registry"}}
		result := MergeSchemas(parent, child)
		req, ok := result["required"].([]any)
		require.True(t, ok, "required should be a slice")
		assert.ElementsMatch(t, []any{"name", "tag", "registry"}, req)
	})

	t.Run("child required preserved when parent has none", func(t *testing.T) {
		parent := map[string]any{"type": "object"}
		child := map[string]any{"required": []any{"registry"}}
		result := MergeSchemas(parent, child)
		req, ok := result["required"].([]any)
		require.True(t, ok, "required should be a slice")
		assert.ElementsMatch(t, []any{"registry"}, req)
	})

	t.Run("nested properties are recursively merged", func(t *testing.T) {
		parent := map[string]any{
			"properties": map[string]any{
				"registry": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"image": map[string]any{"type": "string", "description": "parent description"},
						"tag":   map[string]any{"type": "string"},
					},
				},
			},
		}
		child := map[string]any{
			"required": []any{"registry"},
			"properties": map[string]any{
				"registry": map[string]any{
					"type":     "object",
					"required": []any{"image"},
					"properties": map[string]any{
						"image": map[string]any{"type": "string", "description": "child description"},
					},
				},
			},
		}
		result := MergeSchemas(parent, child)

		req, ok := result["required"].([]any)
		require.True(t, ok, "required should be a slice")
		assert.ElementsMatch(t, []any{"registry"}, req)

		props, ok := result["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		registry, ok := props["registry"].(map[string]any)
		require.True(t, ok, "registry should be a map")

		regReq, ok := registry["required"].([]any)
		require.True(t, ok, "registry.required should be a slice")
		assert.ElementsMatch(t, []any{"image"}, regReq)

		regProps, ok := registry["properties"].(map[string]any)
		require.True(t, ok, "registry.properties should be a map")

		image, ok := regProps["image"].(map[string]any)
		require.True(t, ok, "image should be a map")
		assert.Equal(t, "parent description", image["description"])

		assert.Contains(t, regProps, "tag")
	})

	t.Run("parent top-level scalar wins", func(t *testing.T) {
		parent := map[string]any{"type": "object", "additionalProperties": false}
		child := map[string]any{"type": "array", "additionalProperties": true}
		result := MergeSchemas(parent, child)
		assert.Equal(t, "object", result["type"])
		assert.Equal(t, false, result["additionalProperties"])
	})

	t.Run("child-only definitions entry is preserved", func(t *testing.T) {
		parent := map[string]any{"definitions": map[string]any{}}
		child := map[string]any{
			"definitions": map[string]any{"Name": map[string]any{"type": "string"}},
		}
		result := MergeSchemas(parent, child)
		defs, ok := result["definitions"].(map[string]any)
		require.True(t, ok, "definitions should be a map")
		assert.Contains(t, defs, "Name", "child-only definition should survive parent override with empty map")
	})

	t.Run("parent wins on same definition key", func(t *testing.T) {
		parent := map[string]any{
			"definitions": map[string]any{"Name": map[string]any{"type": "integer"}},
		}
		child := map[string]any{
			"definitions": map[string]any{"Name": map[string]any{"type": "string"}},
		}
		result := MergeSchemas(parent, child)
		defs, ok := result["definitions"].(map[string]any)
		require.True(t, ok)
		name, ok := defs["Name"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "integer", name["type"], "parent definition should win on same key")
	})

	t.Run("$defs is merged the same way as definitions", func(t *testing.T) {
		parent := map[string]any{"$defs": map[string]any{}}
		child := map[string]any{
			"$defs": map[string]any{"ID": map[string]any{"type": "string"}},
		}
		result := MergeSchemas(parent, child)
		defs, ok := result["$defs"].(map[string]any)
		require.True(t, ok, "$defs should be a map")
		assert.Contains(t, defs, "ID")
	})
}
