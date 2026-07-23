// Copyright 2015-2017 Piprate Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ld

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
)

// IsKeyword returns whether or not the given value is a keyword.
func IsKeyword(key interface{}) bool {
	if _, isString := key.(string); !isString {
		return false
	}
	return key == "@base" || key == "@container" || key == "@context" || key == "@default" || key == "@direction" ||
		key == "@embed" || key == "@explicit" || key == "@json" || key == "@id" || key == "@included" ||
		key == "@index" || key == "@first" || key == "@graph" || key == "@import" || key == "@language" ||
		key == "@list" || key == "@nest" || key == "@none" || key == "@omitDefault" || key == "@prefix" ||
		key == "@preserve" || key == "@propagate" || key == "@protected" || key == "@requireAll" ||
		key == "@reverse" || key == "@set" || key == "@type" || key == "@value" || key == "@version" ||
		key == "@vocab"
}

// DeepCompare returns true if v1 equals v2.
func DeepCompare(v1 interface{}, v2 interface{}, listOrderMatters bool) bool {
	if v1 == nil {
		return v2 == nil
	} else if v2 == nil {
		return v1 == nil
	}

	m1, isMap1 := v1.(map[string]interface{})
	m2, isMap2 := v2.(map[string]interface{})
	l1, isList1 := v1.([]interface{})
	l2, isList2 := v2.([]interface{})
	if isMap1 && isMap2 {
		if len(m1) != len(m2) {
			return false
		}
		for _, key := range GetKeys(m1) {
			if val2, present := m2[key]; !present || !DeepCompare(m1[key], val2, listOrderMatters) {
				return false
			}
		}
		return true
	} else if isList1 && isList2 {
		if len(l1) != len(l2) {
			return false
		}
		// used to mark members of l2 that we have already matched to avoid
		// matching the same item twice for lists that have duplicates
		alreadyMatched := make([]bool, len(l2))
		for i := 0; i < len(l1); i++ {
			o1 := l1[i]
			gotMatch := false
			if listOrderMatters {
				gotMatch = DeepCompare(o1, l2[i], listOrderMatters)
			} else {
				for j := 0; j < len(l2); j++ {
					if !alreadyMatched[j] && DeepCompare(o1, l2[j], listOrderMatters) {
						alreadyMatched[j] = true
						gotMatch = true
						break
					}
				}
			}
			if !gotMatch {
				return false
			}
		}
		return true
	} else {
		if v1 != v2 {
			// perform additional checks. If the client code sets UseNumber() property
			// of json.Decoder to decode numbers (see https://golang.org/pkg/encoding/json/#Decoder.UseNumber ),
			// simple comparison will fail.
			return normalizeValue(v1) == normalizeValue(v2)
		} else {
			return true
		}
	}
}

// normalizeValue allows comparisons between json.Number and float/integer values.
func normalizeValue(v interface{}) string {
	floatVal, isFloat := v.(float64)

	if !isFloat {
		if number, isNumber := v.(json.Number); isNumber {
			var floatErr error
			floatVal, floatErr = number.Float64()
			if floatErr == nil {
				isFloat = true
			}
		}
	}
	if isFloat {
		return fmt.Sprintf("%f", floatVal)
	} else {
		return fmt.Sprintf("%s", v)
	}
}

func deepContains(values []interface{}, value interface{}) bool {
	for _, item := range values {
		if DeepCompare(item, value, false) {
			return true
		}
	}
	return false
}

// MergeValue adds a value to a subject. If the value is an array, all values in the array will be added.
func MergeValue(obj map[string]interface{}, key string, value interface{}) {
	if obj == nil {
		return
	}
	values, hasValues := obj[key].([]interface{})
	if !hasValues {
		values = make([]interface{}, 0)

	}
	valueMap, isMap := value.(map[string]interface{})
	_, valueContainsList := valueMap["@list"]
	if key == "@list" || (isMap && valueContainsList) || !deepContains(values, value) {
		values = append(values, value)
	}
	obj[key] = values
}

// IsAbsoluteIri returns true if the given value is an absolute IRI, false if not.
func IsAbsoluteIri(value string) bool {
	if strings.HasPrefix(value, "_:") {
		return true
	}

	u, err := url.Parse(value)
	return err == nil && u.IsAbs()
}

// IsSubject returns true if the given value is a subject with properties.
//
// Note: A value is a subject if all of these hold true:
// 1. It is an Object.
// 2. It is not a @value, @set, or @list.
// 3. It has more than 1 key OR any existing key is not @id.
func IsSubject(v interface{}) bool {
	vMap, isMap := v.(map[string]interface{})
	_, containsValue := vMap["@value"]
	_, containsSet := vMap["@set"]
	_, containsList := vMap["@list"]
	_, containsID := vMap["@id"]
	if isMap && !(containsValue || containsSet || containsList) {
		return len(vMap) > 1 || !containsID
	}
	return false
}

// IsSubjectReference returns true if the given value is a subject reference.
//
// Note: A value is a subject reference if all of these hold True:
// 1. It is an Object.
// 2. It has a single key: @id.
func IsSubjectReference(v interface{}) bool {
	// Note: A value is a subject reference if all of these hold true:
	// 1. It is an Object.
	// 2. It has a single key: @id.
	vMap, isMap := v.(map[string]interface{})
	_, containsID := vMap["@id"]
	return isMap && len(vMap) == 1 && containsID
}

// IsList returns true if the given value is a @list.
func IsList(v interface{}) bool {
	vMap, isMap := v.(map[string]interface{})
	_, hasList := vMap["@list"]
	return isMap && hasList
}

// IsGraph returns true if the given value is a graph.
//
// Note: A value is a graph if all of these hold true:
// 1. It is an object.
// 2. It has an `@graph` key.
// 3. It may have '@id' or '@index'
func IsGraph(v interface{}) bool {
	vMap, isMap := v.(map[string]interface{})
	_, containsGraph := vMap["@graph"]
	hasOtherKeys := false
	if isMap {
		for k := range vMap {
			if k != "@id" && k != "@index" && k != "@graph" {
				hasOtherKeys = true
				break
			}
		}
	}
	return isMap && containsGraph && !hasOtherKeys
}

// IsSimpleGraph returns true if the given value is a simple @graph
func IsSimpleGraph(v interface{}) bool {
	vMap, _ := v.(map[string]interface{})
	_, containsID := vMap["@id"]
	return IsGraph(v) && !containsID
}

// IsRelativeIri returns true if the given value is a relative IRI, false if not.
func IsRelativeIri(value string) bool {
	return !(IsKeyword(value) || IsAbsoluteIri(value))
}

// IsValue returns true if the given value is a JSON-LD value
func IsValue(v interface{}) bool {
	vMap, isMap := v.(map[string]interface{})
	_, containsValue := vMap["@value"]
	return isMap && containsValue
}

// Arrayify returns v, if v is an array, otherwise returns an array
// containing v as the only element.
func Arrayify(v interface{}) []interface{} {
	av, isArray := v.([]interface{})
	if isArray {
		return av
	} else {
		return []interface{}{v}
	}
}

// IsBlankNode returns true if the given value is a blank node.
func IsBlankNodeValue(v interface{}) bool {
	// Note: A value is a blank node if all of these hold true:
	// 1. It is an Object.
	// 2. If it has an @id key its value begins with '_:'.
	// 3. It has no keys OR is not a @value, @set, or @list.
	vMap, isMap := v.(map[string]interface{})
	if isMap {
		id, containsID := vMap["@id"]
		if containsID {
			return strings.HasPrefix(id.(string), "_:")
		} else {
			_, containsValue := vMap["@value"]
			_, containsSet := vMap["@set"]
			_, containsList := vMap["@list"]
			return len(vMap) == 0 || !containsValue || containsSet || containsList
		}
	}
	return false
}

// CompareShortestLeast compares two strings first based on length and then lexicographically.
func CompareShortestLeast(a string, b string) bool {
	if len(a) < len(b) {
		return true
	} else if len(a) > len(b) {
		return false
	} else {
		return a < b
	}
}

// ShortestLeast is a struct which allows sorting using CompareShortestLeast function.
type ShortestLeast []string

func (s ShortestLeast) Len() int {
	return len(s)
}
func (s ShortestLeast) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ShortestLeast) Less(i, j int) bool {
	return CompareShortestLeast(s[i], s[j])
}

func inArray(v interface{}, array []interface{}) bool {
	for _, x := range array {
		if v == x {
			return true
		}
	}
	return false
}

func isEmptyObject(v interface{}) bool {
	vMap, isMap := v.(map[string]interface{})
	return isMap && len(vMap) == 0
}

// RemovePreserve removes the @preserve keywords as the last step of the framing algorithm.
//
// ctx: the active context used to compact the input
// input: the framed, compacted output
// bnodesToClear: list of bnodes to be pruned
// compactArrays: compactArrays flag
//
// Returns the resulting output.
func RemovePreserve(ctx *Context, input interface{}, bnodesToClear []string, compactArrays bool) (interface{}, error) {

	// recurse through arrays
	switch v := input.(type) {
	case []interface{}:
		output := make([]interface{}, 0)
		for _, i := range v {
			result, err := RemovePreserve(ctx, i, bnodesToClear, compactArrays)
			if err != nil {
				return nil, err
			}
			// drop nulls from arrays
			if result != nil {
				output = append(output, result)
			}
		}
		input = output
	case map[string]interface{}:

		// remove @preserve
		if preserveVal, present := v["@preserve"]; present {
			if preserveVal == "@null" {
				return nil, nil
			}
			return preserveVal, nil
		}

		// skip @values
		if _, hasValue := v["@value"]; hasValue {
			return input, nil
		}

		// recurse through @lists
		if listVal, hasList := v["@list"]; hasList {
			var err error
			v["@list"], err = RemovePreserve(ctx, listVal, bnodesToClear, compactArrays)
			if err != nil {
				return nil, err
			}
			return input, nil
		}

		// potentially remove the id, if it is an unreference bnode
		idAlias, err := ctx.CompactIri("@id", nil, false, false)
		if err != nil {
			return nil, err
		}
		if id, hasID := v[idAlias]; hasID {
			for _, bnode := range bnodesToClear {
				if id == bnode {
					delete(v, idAlias)
				}
			}
		}
		// recurse through properties
		graphAlias, err := ctx.CompactIri("@graph", nil, false, false)
		if err != nil {
			return nil, err
		}
		for prop, propVal := range v {
			result, err := RemovePreserve(ctx, propVal, bnodesToClear, compactArrays)
			if err != nil {
				return nil, err
			}
			isListContainer := ctx.HasContainerMapping(prop, "@list")
			isSetContainer := ctx.HasContainerMapping(prop, "@set")
			resultList, isList := result.([]interface{})
			if compactArrays && isList && len(resultList) == 1 && !isSetContainer && !isListContainer && prop != graphAlias {
				result = resultList[0]
			}
			v[prop] = result
		}
	}

	return input, nil
}

// HasValue determines if the given value is a property of the given subject
func HasValue(subject interface{}, property string, value interface{}) bool {

	if subjMap, isMap := subject.(map[string]interface{}); isMap {
		if val, found := subjMap[property]; found {
			isList := IsList(val)
			if valArray, isArray := val.([]interface{}); isArray || isList {
				if isList {
					valArray = val.(map[string]interface{})["@list"].([]interface{})
				}
				for _, v := range valArray {
					if CompareValues(value, v) {
						return true
					}
				}
			} else if _, isArray := value.([]interface{}); !isArray {
				// avoid matching the set of values with an array value parameter
				return CompareValues(value, val)
			}
		}
	}
	return false
}

// AddValue adds a value to a subject. If the value is an array, all values in the
// array will be added.
//
// Options:
//
//	[propertyIsArray] True if the property is always an array, False if not (default: False).
//	[allowDuplicate] True to allow duplicates, False not to (uses a simple shallow comparison
//			of subject ID or value) (default: True).
func AddValue(subject interface{}, property string, value interface{}, propertyIsArray, valueAsArray, allowDuplicate,
	prependValue bool) {

	subjMap, _ := subject.(map[string]interface{})
	propVal, propertyFound := subjMap[property]
	if valueAsArray {
		subjMap[property] = value
	} else if valueArray, isArray := value.([]interface{}); isArray {
		if prependValue {
			if propertyIsArray {
				valueArray = append(subjMap[property].([]interface{}), valueArray...)
			} else {
				valueArray = append([]interface{}{subjMap[property]}, valueArray...)
			}
			subjMap[property] = make([]interface{}, 0)
		} else if len(valueArray) == 0 && propertyIsArray && !propertyFound {
			subjMap[property] = make([]interface{}, 0)
		}
		for _, v := range valueArray {
			AddValue(subject, property, v, propertyIsArray, valueAsArray, allowDuplicate, prependValue)
		}
	} else if propertyFound {
		// check if subject already has value if duplicates not allowed
		hasValue := !allowDuplicate && HasValue(subject, property, value)

		// make property an array if value not present or always an array
		valArray, isArray := propVal.([]interface{})
		if !isArray && (!hasValue || propertyIsArray) {
			valArray = []interface{}{subjMap[property]}
			subjMap[property] = valArray
		}

		// add new value
		if !hasValue {
			if prependValue {
				subjMap[property] = append([]interface{}{value}, valArray...)
			} else {
				subjMap[property] = append(valArray, value)
			}
		}
	} else if propertyIsArray {
		subjMap[property] = []interface{}{value}
	} else {
		subjMap[property] = value
	}
}

// RemoveValue removes a value from a subject.
func RemoveValue(subject interface{}, property string, value interface{}, propertyIsArray bool) {
	subjMap, _ := subject.(map[string]interface{})
	propVal, propertyFound := subjMap[property]
	if !propertyFound {
		return
	}

	values := make([]interface{}, 0)
	for _, v := range Arrayify(propVal) {
		if !CompareValues(v, value) {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		delete(subjMap, property)
	} else if len(values) == 1 && !propertyIsArray {
		subjMap[property] = values[0]
	} else {
		subjMap[property] = values
	}
}

// CompareValues compares two JSON-LD values for equality.
// Two JSON-LD values will be considered equal if:
//
// 1. They are both primitives of the same type and value.
// 2. They are both @values with the same @value, @type, and @language, OR
// 3. They both have @ids they are the same.
func CompareValues(v1 interface{}, v2 interface{}) bool {
	v1Map, isv1Map := v1.(map[string]interface{})
	v2Map, isv2Map := v2.(map[string]interface{})

	if !isv1Map && !isv2Map && v1 == v2 {
		return true
	}

	if IsValue(v1) && IsValue(v2) {
		if v1Map["@value"] == v2Map["@value"] &&
			v1Map["@type"] == v2Map["@type"] &&
			v1Map["@language"] == v2Map["@language"] &&
			v1Map["@index"] == v2Map["@index"] {
			return true
		}
	}

	id1, v1containsID := v1Map["@id"]
	id2, v2containsID := v2Map["@id"]
	if (isv1Map && v1containsID) && (isv2Map && v2containsID) && (id1 == id2) {
		return true
	}

	return false
}

// CloneDocument returns a cloned instance of the given document
func CloneDocument(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	m, isMap := value.(map[string]interface{})
	l, isList := value.([]interface{})

	if isMap {
		mClone := make(map[string]interface{}, len(m))
		for k, v := range m {
			mClone[k] = CloneDocument(v)
		}
		return mClone
	} else if isList {
		lClone := make([]interface{}, 0, len(l))
		for _, v := range l {
			lClone = append(lClone, CloneDocument(v))
		}
		return lClone
	} else {
		// This is a bit simplistic. Beware of string values, at least.
		return value
	}
}

// GetKeys returns all keys in the given object
func GetKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	return keys
}

// GetOrderedKeys returns all keys in the given object as a sorted list
func GetOrderedKeys(m map[string]interface{}) []string {
	keys := GetKeys(m)
	sort.Strings(keys)

	return keys
}

// PrintDocument prints a JSON-LD document. This is useful for debugging.
func PrintDocument(msg string, doc interface{}) {
	b, _ := json.MarshalIndent(doc, "", "  ")
	if msg != "" {
		_, _ = os.Stdout.WriteString(msg)
		_, _ = os.Stdout.WriteString("\n")
	}
	_, _ = os.Stdout.Write(b)
	_, _ = os.Stdout.WriteString("\n")
}
