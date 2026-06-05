// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

// GenerateJSONSchema infers a JSON schema from the structure and scalar types in values.
func GenerateJSONSchema(vals Values) map[string]any {
	props := make(map[string]any)
	schema := map[string]any{
		"$schema":    "http://json-schema.org/draft-07/schema#",
		"type":       "object",
		"properties": props,
	}

	for k, v := range vals {
		props[k] = inferSchemaType(v)
	}

	return schema
}

// ReconcileJSONSchema updates structural fields in an existing schema from inferred values.
// Non-structural fields (description/enum/required/etc.) are preserved by default.
func ReconcileJSONSchema(existing, inferred map[string]any) map[string]any {
	typeVal, hasType := inferred["type"]
	if hasType {
		existing["type"] = typeVal
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		typeStr = ""
	}
	if typeStr == "object" {
		reconcileSchemaProperties(existing, inferred)
	}

	if typeStr == "array" {
		reconcileSchemaItems(existing, inferred)
	}

	if schemaURI, ok := inferred["$schema"]; ok {
		existing["$schema"] = schemaURI
	}

	return existing
}

func reconcileSchemaProperties(existing, inferred map[string]any) {
	inferredProps, ok := inferred["properties"].(map[string]any)
	if !ok {
		return
	}

	existingProps, ok := existing["properties"].(map[string]any)
	if !ok {
		existingProps = make(map[string]any)
		existing["properties"] = existingProps
	}

	for key := range existingProps {
		if _, found := inferredProps[key]; !found {
			delete(existingProps, key)
		}
	}

	for key, inferredProp := range inferredProps {
		inferredPropMap, ok := inferredProp.(map[string]any)
		if !ok {
			existingProps[key] = inferredProp
			continue
		}

		existingPropMap, ok := existingProps[key].(map[string]any)
		if !ok {
			existingProps[key] = inferredPropMap
			continue
		}

		existingProps[key] = ReconcileJSONSchema(existingPropMap, inferredPropMap)
	}
}

func reconcileSchemaItems(existing, inferred map[string]any) {
	inferredItems, hasInferredItems := inferred["items"].(map[string]any)
	if !hasInferredItems {
		return
	}

	existingItems, hasExistingItems := existing["items"].(map[string]any)
	if !hasExistingItems {
		existing["items"] = inferredItems
		return
	}

	existing["items"] = ReconcileJSONSchema(existingItems, inferredItems)
}

func inferSchemaType(v any) any {
	switch val := v.(type) {
	case string:
		return map[string]any{"type": "string"}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return map[string]any{"type": "number"}
	case bool:
		return map[string]any{"type": "boolean"}
	case map[string]any:
		objProps := make(map[string]any)
		for k, v := range val {
			objProps[k] = inferSchemaType(v)
		}
		return map[string]any{
			"type":       "object",
			"properties": objProps,
		}
	case []any:
		if len(val) > 0 {
			return map[string]any{"type": "array", "items": inferSchemaType(val[0])}
		}
		return map[string]any{"type": "array"}
	default:
		return map[string]any{"type": "string"}
	}
}
