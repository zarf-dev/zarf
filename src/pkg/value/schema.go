// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

// LoadJSONSchema reads a JSON schema file if present.
func LoadJSONSchema(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to read existing schema file: %w", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, fmt.Errorf("unable to parse existing schema file: %w", err)
	}

	return schema, nil
}

// CheckNoExternalRefs returns an error if the schema contains any external reference
// pointers ($ref, $dynamicRef, $recursiveRef). Internal fragment references that start
// with "#" — such as "#/definitions/Foo" or "#/$defs/Foo" — are allowed because the
// referenced definition is part of the same document and travels with the schema during
// merge and assembly. External references (relative file paths, HTTP URIs) are rejected
// because the referenced files are not bundled into the assembled package.
func CheckNoExternalRefs(schema map[string]any) error {
	return checkNoExternalRefsInObject(schema)
}

var externalRefKeywords = []string{"$ref", "$dynamicRef", "$recursiveRef"}

func checkNoExternalRefsInObject(node map[string]any) error {
	for _, kw := range externalRefKeywords {
		if val, has := node[kw]; has {
			if ref, ok := val.(string); ok && !strings.HasPrefix(ref, "#") {
				return fmt.Errorf("schema contains an external %q pointer %q; only internal references (\"#/...\") are supported — external files are not bundled into the assembled package", kw, ref)
			}
		}
	}
	for _, key := range slices.Sorted(maps.Keys(node)) {
		if err := checkNoExternalRefsInValue(key, node[key]); err != nil {
			return err
		}
	}
	return nil
}

func checkNoExternalRefsInValue(key string, val any) error {
	switch v := val.(type) {
	case map[string]any:
		if err := checkNoExternalRefsInObject(v); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	case []any:
		for i, item := range v {
			if err := checkNoExternalRefsInValue(fmt.Sprintf("%s[%d]", key, i), item); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyValue returns a deep copy of a value containing only the types produced
// by json.Unmarshal: map[string]any, []any, and scalar primitives.
func copyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return copyMap(val)
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = copyValue(item)
		}
		return cp
	default:
		return v
	}
}

// copyMap returns a deep copy of a map[string]any.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = copyValue(v)
	}
	return out
}

// MergeSchemas merges child into parent with parent-wins semantics and returns a new map.
// Neither parent nor child is modified. Rules:
//   - "properties", "definitions", "$defs", "patternProperties", "dependentSchemas":
//     all are maps of string→schema and are recursively merged; parent wins on same key,
//     child-only entries are preserved so internal $ref pointers remain valid
//   - "required": union of both arrays, deduplicated
//   - all other keys: parent wins (child value used only when key absent from parent)
func MergeSchemas(parent, child map[string]any) map[string]any {
	result := copyMap(parent)
	for key, childVal := range child {
		switch key {
		case "properties", "definitions", "$defs", "patternProperties", "dependentSchemas":
			result[key] = mergeProperties(result[key], childVal)
		case "required":
			if req := mergeRequired(result["required"], childVal); len(req) > 0 {
				result["required"] = req
			}
		default:
			if _, exists := result[key]; !exists {
				result[key] = copyValue(childVal)
			}
		}
	}
	return result
}

func mergeProperties(parentVal, childVal any) any {
	parentProps, parentOk := parentVal.(map[string]any)
	childProps, childOk := childVal.(map[string]any)

	if !childOk {
		return parentVal
	}
	if !parentOk {
		return childVal
	}

	for key, childProp := range childProps {
		if _, exists := parentProps[key]; !exists {
			parentProps[key] = copyValue(childProp)
		} else {
			parentProp, parentPropIsMap := parentProps[key].(map[string]any)
			childPropMap, childPropIsMap := childProp.(map[string]any)
			if parentPropIsMap && childPropIsMap {
				parentProps[key] = MergeSchemas(parentProp, childPropMap)
			}
		}
	}
	return parentProps
}

func mergeRequired(parentVal, childVal any) []any {
	toSlice := func(v any) []any {
		s, ok := v.([]any)
		if !ok {
			return nil
		}
		return s
	}

	ps, cs := toSlice(parentVal), toSlice(childVal)
	seen := make(map[string]struct{})
	var merged []any

	for _, src := range [][]any{ps, cs} {
		for _, item := range src {
			if str, ok := item.(string); ok {
				if _, exists := seen[str]; !exists {
					seen[str] = struct{}{}
					merged = append(merged, str)
				}
			}
		}
	}
	return merged
}
