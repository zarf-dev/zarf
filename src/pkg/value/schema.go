// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package value

import "fmt"

// CheckNoRefs returns an error if the schema object contains any "$ref" pointers.
// Schemas with "$ref" cannot be safely merged because references may point to files
// that are unavailable after package assembly.
func CheckNoRefs(schema map[string]any) error {
	return checkNoRefsInObject(schema)
}

func checkNoRefsInObject(node map[string]any) error {
	if _, hasRef := node["$ref"]; hasRef {
		return fmt.Errorf("schema contains a \"$ref\" pointer which is not supported in imported schemas; flatten the schema before importing")
	}
	for key, val := range node {
		switch v := val.(type) {
		case map[string]any:
			if err := checkNoRefsInObject(v); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		case []any:
			for i, item := range v {
				if m, ok := item.(map[string]any); ok {
					if err := checkNoRefsInObject(m); err != nil {
						return fmt.Errorf("%s[%d]: %w", key, i, err)
					}
				}
			}
		}
	}
	return nil
}

// MergeSchemas merges child into parent with parent-wins semantics. The parent map is
// modified in place and returned. Rules:
//   - "properties": recursively merged; parent wins on same key
//   - "required": union of both arrays, deduplicated
//   - all other keys: parent wins (child value used only when key absent from parent)
func MergeSchemas(parent, child map[string]any) map[string]any {
	for key, childVal := range child {
		switch key {
		case "properties":
			parent["properties"] = mergeProperties(parent["properties"], childVal)
		case "required":
			parent["required"] = mergeRequired(parent["required"], childVal)
		default:
			if _, exists := parent[key]; !exists {
				parent[key] = childVal
			}
		}
	}
	return parent
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
	toStrings := func(v any) []string {
		s, ok := v.([]any)
		if !ok {
			return nil
		}
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	seen := map[string]bool{}
	merged := []any{}

	for _, item := range toStrings(parentVal) {
		if !seen[item] {
			seen[item] = true
			merged = append(merged, item)
		}
	}
	for _, item := range toStrings(childVal) {
		if !seen[item] {
			seen[item] = true
			merged = append(merged, item)
		}
	}
	if len(merged) == 0 {
		return parentVal
	}
	return merged
}
