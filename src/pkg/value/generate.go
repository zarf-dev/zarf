// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

import (
	"fmt"
)

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

// ReconcileJSONSchema updates inferred fields in an existing schema. Existing
// fields take precedence for non-structural validation fields while inferred
// structure and missing validation fields are carried forward.
func ReconcileJSONSchema(existing, inferred map[string]any, deleteNotFound bool) map[string]any {
	existing = copyMap(existing)
	typeVal, hasType := inferred["type"]
	if hasType {
		existing["type"] = typeVal
	}

	if schemaTypeIncludes(typeVal, "object") {
		reconcileSchemaProperties(existing, inferred, deleteNotFound)
	}

	if schemaTypeIncludes(typeVal, "array") {
		reconcileSchemaItems(existing, inferred, deleteNotFound)
	}

	if schemaURI, ok := inferred["$schema"]; ok {
		existing["$schema"] = schemaURI
	}

	// Preserve explicitly-authored fields while carrying inferred validation
	// fields into schemas that do not define them. This lets chart schemas add
	// constraints without overriding package-owned constraints.
	for key, inferredValue := range inferred {
		switch key {
		case "$schema", "type", "properties", "items":
			continue
		}
		if _, exists := existing[key]; !exists {
			existing[key] = copyValue(inferredValue)
		}
	}

	return existing
}

func schemaTypeIncludes(typeVal any, wanted string) bool {
	switch val := typeVal.(type) {
	case string:
		return val == wanted
	case []any:
		for _, item := range val {
			if item == wanted {
				return true
			}
		}
	case []string:
		for _, item := range val {
			if item == wanted {
				return true
			}
		}
	}
	return false
}

// ExtractJSONSchema returns the schema object at a JSON value path. The root
// path (".") returns the supplied schema itself.
func ExtractJSONSchema(schema map[string]any, path Path) (map[string]any, bool, error) {
	if err := path.Validate(); err != nil {
		return nil, false, err
	}
	if path == "." {
		return schema, true, nil
	}

	current := schema
	for _, part := range path.Segments() {
		child, ok := schemaChild(current, part)
		if !ok {
			return nil, false, nil
		}
		current = child
	}
	return current, true, nil
}

// MergeJSONSchemaAtPath overlays a chart schema at a JSON value path. Chart
// fields are copied into the inferred schema; authored package schemas are
// reconciled later and take precedence over conflicting fields.
func MergeJSONSchemaAtPath(schema map[string]any, path Path, overlay map[string]any) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if path == "." {
		mergeChartSchema(schema, overlay)
		return nil
	}

	current := schema
	parts := path.Segments()
	for i, part := range parts {
		child, ok := schemaChild(current, part)
		if !ok {
			return fmt.Errorf("schema path %s: key %q is not an object schema", path, part)
		}
		if i == len(parts)-1 {
			mergeChartSchema(child, overlay)
			return nil
		}
		current = child
	}
	return nil
}

// DeleteJSONSchemaAtPath removes a property schema at a JSON value path. A
// missing path is treated as a no-op, matching Values.Delete behavior.
func DeleteJSONSchemaAtPath(schema map[string]any, path Path) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if path == "." {
		return fmt.Errorf("cannot delete root schema")
	}

	current := schema
	parts := path.Segments()
	for i, part := range parts {
		properties, ok := current["properties"].(map[string]any)
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			delete(properties, part)
			return nil
		}
		child, ok := properties[part].(map[string]any)
		if !ok {
			return nil
		}
		current = child
	}
	return nil
}

func schemaChild(schema map[string]any, part string) (map[string]any, bool) {
	if properties, ok := schema["properties"].(map[string]any); ok {
		if child, ok := properties[part].(map[string]any); ok {
			return child, true
		}
	}

	// A chart schema commonly describes arbitrary map keys through
	// additionalProperties rather than enumerating them in properties.
	if additionalProperties, ok := schema["additionalProperties"].(map[string]any); ok {
		return additionalProperties, true
	}

	return nil, false
}

func mergeChartSchema(destination, source map[string]any) {
	for key, sourceValue := range FilterChartSchema(source) {
		switch key {
		case "properties":
			sourceProperties, ok := sourceValue.(map[string]any)
			if !ok {
				destination[key] = copyValue(sourceValue)
				continue
			}
			destinationProperties, ok := destination[key].(map[string]any)
			if !ok {
				destination[key] = copyValue(sourceValue)
				continue
			}
			for propertyName, sourceProperty := range sourceProperties {
				sourcePropertyMap, sourceIsMap := sourceProperty.(map[string]any)
				destinationPropertyMap, destinationIsMap := destinationProperties[propertyName].(map[string]any)
				if sourceIsMap && destinationIsMap {
					mergeChartSchema(destinationPropertyMap, sourcePropertyMap)
				} else {
					destinationProperties[propertyName] = copyValue(sourceProperty)
				}
			}
		case "items", "additionalProperties":
			sourceMap, sourceIsMap := sourceValue.(map[string]any)
			destinationMap, destinationIsMap := destination[key].(map[string]any)
			if sourceIsMap && destinationIsMap {
				mergeChartSchema(destinationMap, sourceMap)
			} else {
				destination[key] = copyValue(sourceValue)
			}
		default:
			// Validation keywords are chart-owned at this stage and should be
			// retained when they describe values supplied by the package.
			destination[key] = copyValue(sourceValue)
		}
	}
}

// FilterChartSchema retains schema keywords that describe value types or
// validation rules, while dropping annotations such as description, title,
// default, and examples from the chart schema. Reference-dependent schema
// nodes are dropped while independent child properties are retained. Presence
// and lower-bound map constraints are intentionally excluded because Helm
// applies chart defaults before validating the final values object.
func FilterChartSchema(schema map[string]any) map[string]any {
	if hasUnsupportedChartSchemaReference(schema) {
		return nil
	}

	filtered := make(map[string]any)
	for key, value := range schema {
		if !isChartSchemaKeyword(key) {
			continue
		}
		if key == "const" || key == "enum" {
			filtered[key] = copyValue(value)
			continue
		}
		// Strip default `additionalProperties=true` to prevent schema bloat
		if key == "additionalProperties" {
			if allowed, ok := value.(bool); ok && allowed {
				continue
			}
		}

		filteredValue, keep := filterChartSchemaValue(key, value)
		if !keep || isEmptyChartSchemaFragment(key, filteredValue) {
			continue
		}
		filtered[key] = filteredValue
	}
	return filtered
}

// isEmptyChartSchemaFragment identifies empty schemas that add no validation
// beyond JSON Schema's defaults.
func isEmptyChartSchemaFragment(key string, value any) bool {
	if key != "items" && key != "properties" && key != "additionalProperties" {
		return false
	}
	schema, ok := value.(map[string]any)
	return ok && len(schema) == 0
}

func filterChartSchemaValue(key string, value any) (any, bool) {
	if key == "properties" {
		if schemas, ok := value.(map[string]any); ok {
			filtered := make(map[string]any, len(schemas))
			for name, schema := range schemas {
				if schemaMap, ok := schema.(map[string]any); ok {
					filteredSchema := FilterChartSchema(schemaMap)
					if len(filteredSchema) == 0 {
						continue
					}
					filtered[name] = filteredSchema
				} else {
					filtered[name] = copyValue(schema)
				}
			}
			return filtered, true
		}
	}

	switch value := value.(type) {
	case map[string]any:
		filtered := FilterChartSchema(value)
		return filtered, len(filtered) > 0
	case []any:
		filtered := make([]any, len(value))
		for i, item := range value {
			filteredItem, keep := filterChartSchemaValue("", item)
			if !keep {
				return nil, false
			}
			filtered[i] = filteredItem
		}
		return filtered, true
	default:
		return copyValue(value), true
	}
}

// hasUnsupportedChartSchemaReference reports whether a schema node contains a
// reference that cannot be retained while filtering that node. Child schemas
// under properties, items, and additionalProperties are handled independently.
// Definitions are intentionally ignored because they are not copied into the
// generated schema; a field that uses one still has its own reference.
func hasUnsupportedChartSchemaReference(schema map[string]any) bool {
	for key, value := range schema {
		if isReferenceKeyword(key) {
			return true
		}

		switch key {
		case "properties", "items", "additionalProperties", "definitions", "$defs":
			continue
		case "additionalItems", "allOf", "anyOf", "oneOf", "not",
			"if", "then", "else", "contains", "propertyNames",
			"patternProperties", "dependencies", "dependentSchemas",
			"prefixItems", "unevaluatedItems", "unevaluatedProperties", "contentSchema":
			if hasJSONSchemaReference(value) {
				return true
			}
		}
	}
	return false
}

// isChartSchemaKeyword keeps the imported Helm schema surface deliberately
// small: basic shape and scalar/array constraints only.
func isChartSchemaKeyword(key string) bool {
	switch key {
	case "additionalProperties", "const", "enum",
		"exclusiveMaximum", "exclusiveMinimum",
		"items", "maxItems", "maxLength", "maximum",
		"minItems", "minLength", "minimum", "pattern",
		"properties", "type":
		return true
	default:
		return false
	}
}

func reconcileSchemaProperties(existing, inferred map[string]any, deleteNotFound bool) {
	inferredProps, ok := inferred["properties"].(map[string]any)
	if !ok {
		return
	}

	existingProps, ok := existing["properties"].(map[string]any)
	if !ok {
		existingProps = make(map[string]any)
		existing["properties"] = existingProps
	}

	if deleteNotFound {
		for key := range existingProps {
			if _, found := inferredProps[key]; !found {
				delete(existingProps, key)
			}
		}
	}

	for key, inferredProp := range inferredProps {
		inferredPropMap, ok := inferredProp.(map[string]any)
		if !ok {
			existingProps[key] = copyValue(inferredProp)
			continue
		}

		existingPropMap, ok := existingProps[key].(map[string]any)
		if !ok {
			existingProps[key] = copyMap(inferredPropMap)
			continue
		}

		existingProps[key] = ReconcileJSONSchema(existingPropMap, inferredPropMap, deleteNotFound)
	}
}

func reconcileSchemaItems(existing, inferred map[string]any, deleteNotFound bool) {
	inferredItems, hasInferredItems := inferred["items"].(map[string]any)
	if !hasInferredItems {
		return
	}

	existingItems, hasExistingItems := existing["items"].(map[string]any)
	if !hasExistingItems {
		existing["items"] = copyMap(inferredItems)
		return
	}

	existing["items"] = ReconcileJSONSchema(existingItems, inferredItems, deleteNotFound)
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
