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

func TestMergeJSONSchemaAtPathPreservesNullableObjects(t *testing.T) {
	schema := GenerateJSONSchema(Values{
		"serviceAccount": map[string]any{
			"server": map[string]any{
				"annotations": nil,
			},
		},
	})

	err := MergeJSONSchemaAtPath(schema, Path(".serviceAccount.server.annotations"), map[string]any{
		"type":       []any{"object", "null"},
		"properties": map[string]any{},
		"required":   []any{"not-imported"},
	})
	require.NoError(t, err)

	annotations, found, err := ExtractJSONSchema(schema, Path(".serviceAccount.server.annotations"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []any{"object", "null"}, annotations["type"])
	assert.NotContains(t, annotations, "properties")
	assert.NotContains(t, annotations, "required")

	existing := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"serviceAccount": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"server": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"annotations": map[string]any{
								"type":        "string",
								"description": "preserve this",
								"required":    []any{"package-owned"},
							},
						},
					},
				},
			},
		},
	}
	result := ReconcileJSONSchema(existing, schema, false)
	annotations, found, err = ExtractJSONSchema(result, Path(".serviceAccount.server.annotations"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []any{"object", "null"}, annotations["type"])
	assert.Equal(t, "preserve this", annotations["description"])
	assert.Equal(t, []any{"package-owned"}, annotations["required"])
}

func TestMergeJSONSchemaAtPathAtMappedObject(t *testing.T) {
	schema := GenerateJSONSchema(Values{
		"backend": map[string]any{
			"configMap": map[string]any{
				"annotations": nil,
			},
		},
	})

	err := MergeJSONSchemaAtPath(schema, Path(".backend"), map[string]any{
		"type": "object",
		"properties": map[string]any{
			"configMap": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"annotations": map[string]any{
						"type": []any{"object", "null"},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	annotations, found, err := ExtractJSONSchema(schema, Path(".backend.configMap.annotations"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, []any{"object", "null"}, annotations["type"])
}

func TestExtractJSONSchemaUsesAdditionalProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
		},
	}

	result, found, err := ExtractJSONSchema(schema, Path(".config.database"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "string", result["type"])
}

func TestMergeJSONSchemaAtPathCopiesValidationFields(t *testing.T) {
	schema := GenerateJSONSchema(Values{
		"config": map[string]any{
			"database": "postgres",
			"ports":    []any{"http"},
		},
	})

	err := MergeJSONSchemaAtPath(schema, Path(".config.database"), map[string]any{
		"type":      "string",
		"minLength": float64(3),
		"enum":      []any{"postgres", "mysql"},
	})
	require.NoError(t, err)

	database, found, err := ExtractJSONSchema(schema, Path(".config.database"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, float64(3), database["minLength"])
	assert.Equal(t, []any{"postgres", "mysql"}, database["enum"])

	err = MergeJSONSchemaAtPath(schema, Path(".config.ports"), map[string]any{
		"type":     "array",
		"minItems": float64(1),
		"maxItems": float64(3),
	})
	require.NoError(t, err)
	ports, found, err := ExtractJSONSchema(schema, Path(".config.ports"))
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, float64(1), ports["minItems"])
	assert.Equal(t, float64(3), ports["maxItems"])

	err = MergeJSONSchemaAtPath(schema, Path(".config"), map[string]any{
		"minProperties": float64(1),
		"required":      []any{"database"},
		"allOf":         []any{map[string]any{"const": "postgres"}},
	})
	require.NoError(t, err)
	config, found, err := ExtractJSONSchema(schema, Path(".config"))
	require.NoError(t, err)
	require.True(t, found)
	assert.NotContains(t, config, "minProperties")
	assert.NotContains(t, config, "required")
	assert.NotContains(t, config, "allOf")
}

func TestMergeJSONSchemaAtPathKeepsInferredFieldWhenChartRefIsDropped(t *testing.T) {
	schema := GenerateJSONSchema(Values{
		"bad": map[string]any{
			"value": "inferred",
		},
	})

	err := MergeJSONSchemaAtPath(schema, Path("."), map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"good": map[string]any{
				"type":      "string",
				"minLength": float64(3),
			},
			"bad": map[string]any{
				"type": "object",
				"$ref": "schemas/external.json",
			},
		},
	})
	require.NoError(t, err)

	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, properties, "good")
	assert.Contains(t, properties, "bad", "the inferred field must remain available")
	assert.Equal(t, "object", properties["bad"].(map[string]any)["type"])
	assert.Equal(t, false, schema["additionalProperties"])
}

func TestFilterChartSchemaDropsUnsupportedChildSchemas(t *testing.T) {
	filtered := FilterChartSchema(map[string]any{
		"properties": map[string]any{
			"descriptionOnly": map[string]any{
				"description": "unsupported chart metadata",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "application name",
			},
		},
	})

	properties, ok := filtered["properties"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, properties, "descriptionOnly")
	assert.Equal(t, map[string]any{"type": "string"}, properties["name"])
}

func TestFilterChartSchemaDropsReferenceDependentChildSchemas(t *testing.T) {
	filtered := FilterChartSchema(map[string]any{
		"properties": map[string]any{
			"good": map[string]any{
				"type":      "string",
				"minLength": float64(3),
			},
			"external": map[string]any{
				"type": "object",
				"$ref": "schemas/external.json",
			},
			"local": map[string]any{
				"$ref": "#/$defs/Shared",
			},
			"composed": map[string]any{
				"type": "object",
				"allOf": []any{
					map[string]any{"$ref": "https://example.com/schema.json"},
				},
			},
			"nested": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"goodChild": map[string]any{"type": "boolean"},
					"badChild":  map[string]any{"$ref": "#/$defs/Child"},
				},
			},
		},
	})

	properties, ok := filtered["properties"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"type": "string", "minLength": float64(3)}, properties["good"])
	assert.NotContains(t, properties, "external")
	assert.NotContains(t, properties, "local")
	assert.NotContains(t, properties, "composed")

	nested, ok := properties["nested"].(map[string]any)
	require.True(t, ok)
	nestedProperties, ok := nested["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, nestedProperties, "goodChild")
	assert.NotContains(t, nestedProperties, "badChild")
}

func TestReconcileJSONSchema(t *testing.T) {
	tests := []struct {
		name           string
		deleteNotFound bool
	}{
		{name: "retains fields not in inferred schema", deleteNotFound: false},
		{name: "removes fields not in inferred schema", deleteNotFound: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

			result := ReconcileJSONSchema(existing, inferred, tc.deleteNotFound)

			props, ok := result["properties"].(map[string]any)
			require.True(t, ok)
			site, ok := props["site"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Site configuration", site["description"]) // preserved
			assert.Equal(t, []any{"name"}, site["required"])           // preserved

			siteProps, ok := site["properties"].(map[string]any)
			require.True(t, ok)
			_, hasLegacy := siteProps["legacy"]
			if tc.deleteNotFound {
				assert.False(t, hasLegacy) // removed (not inferred)
			} else {
				assert.True(t, hasLegacy) // retained
			}

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
			if tc.deleteNotFound {
				assert.False(t, hasOldField) // removed (not inferred)
			} else {
				assert.True(t, hasOldField) // retained
			}
		})
	}
}
