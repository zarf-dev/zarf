// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// CheckNoRefs returns an error if the schema object contains any "$ref" pointers.
// "$ref" is not supported because referenced files are not bundled into the assembled
// package and cannot be resolved after assembly. Flatten the schema into a single
// self-contained file before use.
func CheckNoRefs(schema map[string]any) error {
	return checkNoRefsInObject(schema)
}

var blockedRefKeywords = []string{"$ref", "$dynamicRef", "$recursiveRef"}

func checkNoRefsInObject(node map[string]any) error {
	for _, kw := range blockedRefKeywords {
		if _, has := node[kw]; has {
			return fmt.Errorf("schema contains a %q pointer; flatten the schema into a single self-contained file", kw)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(node)) {
		if err := checkNoRefsInValue(key, node[key]); err != nil {
			return err
		}
	}
	return nil
}

func checkNoRefsInValue(key string, val any) error {
	switch v := val.(type) {
	case map[string]any:
		if err := checkNoRefsInObject(v); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	case []any:
		for i, item := range v {
			if err := checkNoRefsInValue(fmt.Sprintf("%s[%d]", key, i), item); err != nil {
				return err
			}
		}
	}
	return nil
}

// MergeSchemas merges child into parent with parent-wins semantics and returns a new map.
// Neither parent nor child is modified. Rules:
//   - "properties": recursively merged; parent wins on same key
//   - "required": union of both arrays, deduplicated
//   - all other keys: parent wins (child value used only when key absent from parent)
func MergeSchemas(parent, child map[string]any) map[string]any {
	// Deep-copy parent via JSON round-trip so the caller's map is never mutated.
	b, _ := json.Marshal(parent)
	var result map[string]any
	_ = json.Unmarshal(b, &result)

	for key, childVal := range child {
		switch key {
		case "properties":
			result["properties"] = mergeProperties(result["properties"], childVal)
		case "required":
			result["required"] = mergeRequired(result["required"], childVal)
		default:
			if _, exists := result[key]; !exists {
				result[key] = childVal
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
			parentProps[key] = childProp
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

func mergeRequired(parentVal, childVal any) any {
	toSlice := func(v any) []any {
		s, ok := v.([]any)
		if !ok {
			return nil
		}
		return s
	}

	ps, cs := toSlice(parentVal), toSlice(childVal)
	seen := make(map[string]struct{}, len(ps)+len(cs))
	merged := make([]any, 0, len(ps)+len(cs))

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
	if len(merged) == 0 {
		return parentVal
	}
	return merged
}
