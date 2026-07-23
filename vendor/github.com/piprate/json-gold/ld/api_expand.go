// Copyright 2015-2017 Piprate Limited
// Copyright 2025 Siemens AG
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
	"fmt"
	"sort"
	"strings"
)

// Expand operation expands the given input according to the steps in the Expansion algorithm:
//
// http://www.w3.org/TR/json-ld-api/#expansion-algorithm
//
// Returns the expanded JSON-LD object.
// Returns an error if there was an error during expansion.
func (api *JsonLdApi) Expand(activeCtx *Context, activeProperty string, element interface{}, opts *JsonLdOptions, insideIndex bool, typeScopedContext *Context) (interface{}, error) {

	frameExpansion := opts.ProcessingMode == JsonLd_1_1_Frame
	// 1)
	if element == nil {
		return nil, nil
	}

	// disable framing if activeProperty is @default
	if activeProperty == "@default" {
		frameExpansion = false
	}

	// 3)
	switch elem := element.(type) {
	case []interface{}:
		// 3.1)
		var resultList = make([]interface{}, 0)
		// 3.2)
		for _, item := range elem {
			// 3.2.1)
			v, err := api.Expand(activeCtx, activeProperty, item, opts, insideIndex, typeScopedContext)
			if err != nil {
				return nil, err
			}

			if activeProperty == "@list" || activeCtx.HasContainerMapping(activeProperty, "@list") {
				_, isList := v.([]interface{})
				if isList {
					// if the active property is @list or its container mapping is set to @list and v is an array,
					// change it to a list object
					v = map[string]interface{}{
						"@list": v,
					}
				}
			}

			if v != nil {
				// 3.2.3)
				vList, isList := v.([]interface{})
				if isList {
					resultList = append(resultList, vList...)
				} else {
					resultList = append(resultList, v)
				}
			}
		}
		// 3.3)
		return resultList, nil

	case map[string]interface{}:

		// first, expand the active property
		expandedActiveProperty, err := activeCtx.ExpandIri(activeProperty, false, true, nil, nil)
		if err != nil {
			return nil, err
		}

		// Get any property-scoped context for activeProperty
		var propertyScopedCtx interface{}
		if td := activeCtx.GetTermDefinition(activeProperty); td != nil {
			propertyScopedCtx = td.context
		}

		// second, determine if any type-scoped context should be reverted; it
		// should only be reverted when the following are all true:
		// 1. element is not a value or subject reference
		// 2. insideIndex is false
		if typeScopedContext == nil && activeCtx.previousContext != nil {
			typeScopedContext = activeCtx
		}

		mustRevert := !insideIndex
		elemOrderedKeys := GetOrderedKeys(elem)
		elemCtx, hasContext := elem["@context"]
		if mustRevert && (typeScopedContext != nil) && len(elemOrderedKeys) <= 2 && !hasContext {
			for _, key := range elemOrderedKeys {
				expandedProperty, err := typeScopedContext.ExpandIri(key, false, true, nil, nil)
				if err != nil {
					return nil, err
				}
				if expandedProperty == "@value" {
					// value found, ensure type-scoped context is used to expand it
					mustRevert = false
					activeCtx = typeScopedContext
					break
				}
				if expandedProperty == "@id" && len(elemOrderedKeys) == 1 {
					mustRevert = false
					break
				}
			}
		}

		if mustRevert {
			activeCtx = activeCtx.RevertToPreviousContext()
		}

		if propertyScopedCtx != nil {
			// apply property-scoped context after reverting term-scoped context
			newCtx, err := activeCtx.parse(propertyScopedCtx, nil, false, true, false, true)
			if err != nil {
				return nil, err
			}
			activeCtx = newCtx
		}

		// if element has a context, process it
		if hasContext {
			newCtx, err := activeCtx.Parse(elemCtx)
			if err != nil {
				return nil, err
			}
			activeCtx = newCtx
		}

		// set the type-scoped context to the context on input, for use later
		typeScopedContext = activeCtx

		var typeKey string
		// look for scoped context on @type
		for _, key := range elemOrderedKeys {
			expandedProperty, err := activeCtx.ExpandIri(key, false, true, nil, nil)
			if err != nil {
				return nil, err
			}
			if expandedProperty == "@type" {
				// set scoped contexts from @type
				types := make([]string, 0)

				switch v := elem[key].(type) {
				case []interface{}:
					for _, t := range v {
						if typeStr, isString := t.(string); isString {
							types = append(types, typeStr)
						} else {
							return nil, NewJsonLdError(InvalidTypeValue,
								"@type value must be a string or array of strings")
						}
					}
					// process in lexicographical order, see https://github.com/json-ld/json-ld.org/issues/616
					sort.Strings(types)
				case string:
					types = append(types, v)
				case map[string]interface{}:
					if !frameExpansion {
						return nil, NewJsonLdError(InvalidTypeValue,
							"@type value must be a string or array of strings")
					}
				default:
					return nil, NewJsonLdError(InvalidTypeValue,
						"@type value must be a string or array of strings")
				}

				for _, tt := range types {
					if td := typeScopedContext.GetTermDefinition(tt); td != nil && td.hasContext {
						newCtx, err := activeCtx.parse(td.context, nil, false, false, false, false)
						if err != nil {
							return nil, err
						}
						activeCtx = newCtx
					}
				}

				typeKey = key
			}
		}

		resultMap := make(map[string]interface{})
		err = api.expandObject(activeCtx, activeProperty, expandedActiveProperty, elem, resultMap, typeKey, opts,
			typeScopedContext, frameExpansion)
		if err != nil {
			return nil, err
		}

		// 8)
		if rval, hasValue := resultMap["@value"]; hasValue {
			// 8.1)
			allowedKeys := map[string]interface{}{
				"@value":     nil,
				"@index":     nil,
				"@language":  nil,
				"@type":      nil,
				"@direction": nil,
			}
			hasDisallowedKeys := false
			for key := range resultMap {
				if _, containsKey := allowedKeys[key]; !containsKey {
					hasDisallowedKeys = true
					break
				}
			}
			_, hasLanguage := resultMap["@language"]
			_, hasDirection := resultMap["@direction"]
			typeValue, hasType := resultMap["@type"]
			if hasDisallowedKeys {
				return nil, NewJsonLdError(InvalidValueObject, "value object has unknown keys")
			}
			if (hasLanguage || hasDirection) && hasType {
				return nil, NewJsonLdError(InvalidValueObject,
					"value object must not include @type with either @language or @direction")
			}
			// 8.2)
			if rval == nil && typeValue != "@json" {
				// nothing else is possible with result if we set it to
				// null, so simply return it
				return nil, nil
			}
			// 8.3)

			if hasLanguage {
				for _, v := range Arrayify(rval) {
					if _, isString := v.(string); !(isString || isEmptyObject(v)) {
						return nil, NewJsonLdError(InvalidLanguageTaggedValue,
							"only strings may be language-tagged")
					}
				}
			} else if hasType {
				types := Arrayify(typeValue)
				if activeCtx.processingMode(1.1) && len(types) == 1 && types[0] == "@json" {
					// Any value of @value is okay if @type: @json
				} else {
					for _, v := range types {
						vStr, isString := v.(string)
						if !(isEmptyObject(v) || (isString && IsAbsoluteIri(vStr) && !strings.HasPrefix(vStr, "_:"))) {
							return nil, NewJsonLdError(InvalidTypedValue,
								"an element containing @value and @type must have an absolute IRI for the value of @type")
						}
					}
				}
			}
		} else if rtype, hasType := resultMap["@type"]; hasType { // 9)
			if _, isList := rtype.([]interface{}); !isList {
				resultMap["@type"] = []interface{}{rtype}
			}
		} else {
			// 10)
			rset, hasSet := resultMap["@set"]
			_, hasList := resultMap["@list"]
			if hasSet || hasList {
				// 10.1)
				maxSize := 1
				if _, hasIndex := resultMap["@index"]; hasIndex {
					maxSize = 2
				}
				if len(resultMap) > maxSize {
					return nil, NewJsonLdError(InvalidSetOrListObject,
						"@set or @list may only contain @index")
				}
				// 10.2)
				if hasSet {
					// result becomes an array here, thus the remaining checks
					// will never be true from here on
					// so simply return the value rather than have to make
					// result an object and cast it with every
					// other use in the function.
					return rset, nil
				}
			}
		}
		// 11)
		if _, hasLanguage := resultMap["@language"]; hasLanguage && len(resultMap) == 1 {
			resultMap = nil
		}
		// 12)
		if activeProperty == "" || activeProperty == "@graph" {
			// 12.1)
			_, hasValue := resultMap["@value"]
			_, hasList := resultMap["@list"]
			_, hasID := resultMap["@id"]
			if resultMap != nil && (len(resultMap) == 0 || hasValue || hasList) {
				resultMap = nil
			} else if resultMap != nil && !frameExpansion && hasID && len(resultMap) == 1 { // 12.2)
				resultMap = nil
			}
		}
		// 13)
		if resultMap != nil {
			return resultMap, nil
		} else {
			return nil, nil
		}
	default:
		// 2) If element is a scalar
		// 2.1)
		if activeProperty == "" || activeProperty == "@graph" {
			return nil, nil
		}
		return activeCtx.ExpandValue(activeProperty, element)
	}
}

func (api *JsonLdApi) expandObject(activeCtx *Context, activeProperty string, expandedActiveProperty string, elem map[string]interface{}, resultMap map[string]interface{}, typeKey string, opts *JsonLdOptions, typeScopedContext *Context, frameExpansion bool) error {
	inputType := elem[typeKey]
	if inputType != nil {
		if itArray, isArray := inputType.([]interface{}); isArray {
			if len(itArray) > 0 {
				inputType = itArray[len(itArray)-1]
			} else {
				inputType = nil
			}
		}
		if _, isMap := inputType.(map[string]interface{}); isMap {
			if frameExpansion {
				inputType = nil
			} else {
				return NewJsonLdError(InvalidTypedValue, "@type value must be a string or array of strings")
			}
		}
		if inputType != nil {
			var err error
			inputType, err = activeCtx.ExpandIri(inputType.(string), false, true, nil, nil)
			if err != nil {
				return err
			}
		}
	}

	// 6)
	nests := make([]string, 0)
	// 7)
	for _, key := range GetOrderedKeys(elem) {
		value := elem[key]
		// 7.1)
		if key == "@context" {
			continue
		}
		// 7.2)
		expandedProperty, err := activeCtx.ExpandIri(key, false, true, nil, nil)
		if err != nil {
			return err
		}
		var expandedValue interface{}
		// 7.3)
		if expandedProperty == "" || (!strings.Contains(expandedProperty, ":") && !IsKeyword(expandedProperty)) {
			if activeCtx.options != nil && activeCtx.options.SafeMode {
				return NewJsonLdError(InvalidProperty, "Dropping property that did not expand into an absolute IRI or keyword.")
			} else {
				continue
			}
		}
		// 7.4)
		if IsKeyword(expandedProperty) {
			// 7.4.1)
			if expandedActiveProperty == "@reverse" {
				return NewJsonLdError(InvalidReversePropertyMap,
					"a keyword cannot be used as a @reverse property")
			}
			// 7.4.2)
			_, containsKey := resultMap[expandedProperty]
			if containsKey && expandedProperty != "@type" && expandedProperty != "@included" {
				return NewJsonLdError(CollidingKeywords, expandedProperty+" already exists in result")
			}
			// 7.4.3)
			if expandedProperty == "@id" {
				valueStr, isString := value.(string)
				if isString {
					expandedValue, err = activeCtx.ExpandIri(valueStr, true, false, nil, nil)
					if err != nil {
						return err
					}
				} else if frameExpansion {
					switch v := value.(type) {
					case map[string]interface{}:
						if len(v) != 0 {
							return NewJsonLdError(InvalidIDValue, "@id value must be a an empty object for framing")
						}
						expandedValue = []interface{}{v}
					case []interface{}:
						expandedValueList := make([]interface{}, 0)
						for _, listVal := range v {
							vString, isString := listVal.(string)
							if !isString {
								return NewJsonLdError(InvalidIDValue, "@id value must be a string, an array of strings or an empty dictionary")
							}
							vString, err = activeCtx.ExpandIri(vString, true, true, nil, nil)
							if err != nil {
								return err
							}
							expandedValueList = append(expandedValueList, vString)
						}
						expandedValue = expandedValueList
					default:
						return NewJsonLdError(InvalidIDValue, "value of @id must be a string, an array of strings or an empty dictionary")
					}
				} else {
					return NewJsonLdError(InvalidIDValue, "value of @id must be a string")
				}
			} else if expandedProperty == "@included" {
				// Included blocks are treated as an array of separate object nodes sharing the same
				// referencing active_property. For 1.0, it is skipped as are other unknown keywords
				if activeCtx.processingMode(1.0) {
					continue
				}

				ev, err := api.Expand(activeCtx, activeProperty, value, opts, false, nil)
				if err != nil {
					return err
				}
				includedResult := Arrayify(ev)
				for _, v := range includedResult {
					if !IsSubject(v) {
						return NewJsonLdError(InvalidIncludedValue,
							"values of @included must expand to node objects")
					}
				}
				if prevIncluded, found := resultMap["@included"]; found {
					includedResult = append(prevIncluded.([]interface{}), includedResult...)
				}
				expandedValue = includedResult
			} else if expandedProperty == "@type" { // 7.4.4)
				switch v := value.(type) {
				case []interface{}:
					var expandedValueList []interface{}
					for _, listElem := range v {
						listElemStr, isString := listElem.(string)
						if !isString {
							return NewJsonLdError(InvalidTypeValue,
								"@type value must be a string or array of strings")
						}
						newVal, err := typeScopedContext.ExpandIri(listElemStr, true, true, nil, nil)
						if err != nil {
							return err
						}
						expandedValueList = append(expandedValueList, newVal)
					}
					expandedValue = expandedValueList
				case string:
					expandedValue, err = typeScopedContext.ExpandIri(v, true, true, nil, nil)
					if err != nil {
						return err
					}
					if containsKey {
						expandedValue = append(Arrayify(resultMap[expandedProperty]), expandedValue)
					}
				case map[string]interface{}:
					if len(v) != 0 {
						return NewJsonLdError(InvalidTypeValue,
							"@type value must be a an empty object for framing")
					}
					expandedValue = value
				default:
					return NewJsonLdError(InvalidTypeValue, v)
				}
			} else if expandedProperty == "@graph" { // 7.4.5)
				expandedValue, err = api.Expand(activeCtx, "@graph", value, opts, false, nil)
				if err != nil {
					return err
				}
				expandedValue = Arrayify(expandedValue)
			} else if expandedProperty == "@value" { // 7.4.6)
				if inputType == "@json" && activeCtx.processingMode(1.1) {
					// allow any value, to be verified when the object is fully expanded and
					// the @type is @json.
				} else {
					_, isMap := value.(map[string]interface{})
					_, isList := value.([]interface{})
					if value != nil && (isMap || isList) && !frameExpansion {
						return NewJsonLdError(InvalidValueObjectValue, "value of "+
							expandedProperty+" must be a scalar or null")
					}

				}

				expandedValue = value
				if expandedValue == nil {
					resultMap["@value"] = nil
					continue
				}
			} else if expandedProperty == "@language" {

				// If expanded property is @language and value is not a string, an invalid language-tagged
				// string error has been detected and processing is aborted. Otherwise, set expanded value
				// to lowercase value.

				if frameExpansion {
					// If framing, always use array form
					expandedValues := make([]interface{}, 0)
					for _, v := range Arrayify(value) {
						if vStr, isString := v.(string); isString {
							expandedValues = append(expandedValues, strings.ToLower(vStr))
						} else {
							expandedValues = append(expandedValues, v)
						}
					}
					expandedValue = expandedValues
				} else {
					vStr, isString := value.(string)
					if !isString {
						return NewJsonLdError(InvalidLanguageTaggedString, "@language value must be a string")
					}
					expandedValue = strings.ToLower(vStr)
				}
			} else if expandedProperty == "@direction" {

				// If expanded property is @direction and value is not either 'ltr' or 'rtl', an invalid
				// base direction error has been detected and processing is aborted. Otherwise, set
				// expanded value to value.

				if frameExpansion {
					// If framing, always use array form
					expandedValues := make([]interface{}, 0)
					for _, v := range Arrayify(value) {
						if vStr, isString := v.(string); isString {
							expandedValues = append(expandedValues, strings.ToLower(vStr))
						} else {
							expandedValues = append(expandedValues, v)
						}
					}
					expandedValue = expandedValues
				} else {
					if _, isString := value.(string); !isString {
						return NewJsonLdError(InvalidBaseDirection, "@direction must be one of 'ltr', 'rtl'")
					}
					expandedValue = value
				}
			} else if expandedProperty == "@index" { // 7.4.8)
				_, isString := value.(string)
				if !isString {
					return NewJsonLdError(InvalidIndexValue, "Value of "+
						expandedProperty+" must be a string")
				}
				expandedValue = value
			} else if expandedProperty == "@list" { // 7.4.9)
				// 7.4.9.1)
				if activeProperty == "" || activeProperty == "@graph" {
					continue
				}
				// 7.4.9.2)
				expandedValue, _ = api.Expand(activeCtx, activeProperty, value, opts, false, nil)

				// NOTE: step not in the spec yet
				expandedValue = Arrayify(expandedValue)

			} else if expandedProperty == "@set" { // 7.4.10)
				expandedValue, _ = api.Expand(activeCtx, activeProperty, value, opts, false, nil)
			} else if expandedProperty == "@reverse" { // 7.4.11)
				_, isMap := value.(map[string]interface{})
				if !isMap {
					return NewJsonLdError(InvalidReverseValue, "@reverse value must be an object")
				}
				// 7.4.11.1)
				expandedValue, err = api.Expand(activeCtx, "@reverse", value, opts, false, nil)
				if err != nil {
					return err
				}

				// NOTE: algorithm assumes the result is a map
				// 7.4.11.2)
				reverseValue, containsReverse := expandedValue.(map[string]interface{})["@reverse"]
				if containsReverse {
					for property, item := range reverseValue.(map[string]interface{}) {
						// 7.4.11.2.1)
						var propertyList []interface{}
						if propertyValue, containsProperty := resultMap[property]; containsProperty {
							propertyList = propertyValue.([]interface{})
						} else {
							propertyList = make([]interface{}, 0)
							resultMap[property] = propertyList
						}
						// 7.4.11.2.2)
						if itemList, isList := item.([]interface{}); isList {
							propertyList = append(propertyList, itemList...)
						} else {
							propertyList = append(propertyList, item)
						}
						resultMap[property] = propertyList
					}
				}
				// 7.4.11.3)
				expandedValueMap := expandedValue.(map[string]interface{})
				var maxSize int
				if containsReverse {
					maxSize = 1
				} else {
					maxSize = 0
				}
				if len(expandedValueMap) > maxSize {
					var reverseMap map[string]interface{}
					if reverseValue, containsReverse := resultMap["@reverse"]; containsReverse {
						// 7.4.11.3.2)
						reverseMap = reverseValue.(map[string]interface{})
					} else {
						// 7.4.11.3.1)
						reverseMap = make(map[string]interface{})
						resultMap["@reverse"] = reverseMap
					}

					// 7.4.11.3.3)
					for property, propertyValue := range expandedValueMap {
						if property == "@reverse" {
							continue
						}
						// 7.4.11.3.3.1)
						items := propertyValue.([]interface{})
						for _, item := range items {
							// 7.4.11.3.3.1.1)
							itemMap := item.(map[string]interface{})
							_, containsValue := itemMap["@value"]
							_, containsList := itemMap["@list"]
							if containsValue || containsList {
								return NewJsonLdError(InvalidReversePropertyValue, nil)
							}
							// 7.4.11.3.3.1.2)
							var propertyValueList []interface{}
							propertyValue, containsProperty := reverseMap[property]
							if containsProperty {
								propertyValueList = propertyValue.([]interface{})
							} else {
								propertyValueList = make([]interface{}, 0)
								reverseMap[property] = propertyValueList
							}
							// 7.4.11.3.3.1.3)
							reverseMap[property] = append(propertyValueList, item)
						}
					}
				}
				// 7.4.11.4)
				continue
			} else if expandedProperty == "@nest" {
				// nested keys
				nests = append(nests, key)
			} else if expandedProperty == "@default" {
				expandedValue, _ = api.Expand(activeCtx, expandedProperty, value, opts, false, nil)
			} else if expandedProperty == "@explicit" ||
				expandedProperty == "@embed" ||
				expandedProperty == "@requireAll" ||
				expandedProperty == "@omitDefault" {
				// these values are scalars
				expandedValue = []interface{}{value}
			}
			// 7.4.12)
			if expandedValue != nil {
				resultMap[expandedProperty] = expandedValue
			}
			// 7.4.13)
			continue
		}

		// use potential scoped context for key
		termCtx := activeCtx
		td := activeCtx.GetTermDefinition(key)
		if td != nil && td.hasContext {
			// TODO: fix calling a private method
			//termCtx, err = activeCtx.Parse(ctx)
			termCtx, err = activeCtx.parse(td.context, make([]string, 0), false, true, false, true)
			if err != nil {
				return err
			}
		}

		valueMap, isMap := value.(map[string]interface{})
		if termCtx.HasContainerMapping(key, "@language") && isMap {
			var expandedValueList []interface{}

			dir, hasDir := td.direction, td.hasDirection

			for _, language := range GetOrderedKeys(valueMap) {
				expandedLanguage, err := termCtx.ExpandIri(language, false, true, nil, nil)
				if err != nil {
					return err
				}
				languageList := Arrayify(valueMap[language])
				for _, item := range languageList {
					if item == nil {
						continue
					}

					if _, isString := item.(string); !isString {
						return NewJsonLdError(InvalidLanguageMapValue,
							fmt.Sprintf("expected %v to be a string", item))
					}

					v := map[string]interface{}{
						"@value": item,
					}
					if expandedLanguage != "@none" {
						v["@language"] = strings.ToLower(language)
					}
					if hasDir {
						if dir != nil {
							v["@direction"] = dir
						}
					} else if defaultDir := termCtx.values.direction; defaultDir != "" {
						v["@direction"] = defaultDir
					}
					expandedValueList = append(expandedValueList, v)
				}
			}
			expandedValue = expandedValueList
		} else if termCtx.HasContainerMapping(key, "@index") && isMap { // 7.6)
			asGraph := termCtx.HasContainerMapping(key, "@graph")
			indexKey := "@index"
			if tdKey := termCtx.GetTermDefinition(key); tdKey != nil && tdKey.index != "" {
				indexKey = tdKey.index
			}
			var propertyIndex string
			if indexKey != "@index" {
				propertyIndex, err = activeCtx.ExpandIri(indexKey, false, true, nil, nil)
				if err != nil {
					return err
				}
			}
			expandedValue, err = api.expandIndexMap(termCtx, key, valueMap, indexKey, asGraph, propertyIndex,
				opts)
			if err != nil {
				return err
			}
		} else if termCtx.HasContainerMapping(key, "@id") && isMap {
			asGraph := termCtx.HasContainerMapping(key, "@graph")
			expandedValue, err = api.expandIndexMap(termCtx, key, valueMap, "@id", asGraph, "",
				opts)
			if err != nil {
				return err
			}
		} else if termCtx.HasContainerMapping(key, "@type") && isMap {
			// since container is @type, revert type scoped context when expanding
			expandedValue, err = api.expandIndexMap(termCtx.RevertToPreviousContext(), key, valueMap, "@type",
				false, "", opts)
			if err != nil {
				return err
			}
		} else {
			isList := expandedProperty == "@list"
			tdKey := activeCtx.GetTermDefinition(key)
			if isList || expandedProperty == "@set" {
				nextActiveProperty := activeProperty
				if isList && expandedActiveProperty == "@graph" {
					nextActiveProperty = ""
				}
				expandedValue, err = api.Expand(termCtx, nextActiveProperty, value, opts, false, nil)
				if err != nil {
					return err
				}
			} else if tdKey != nil && tdKey.typ == "@json" {
				expandedValue = map[string]interface{}{
					"@type":  "@json",
					"@value": value,
				}
			} else {
				// 7.7)
				expandedValue, err = api.Expand(termCtx, key, value, opts, false, nil)
				if err != nil {
					return err
				}
			}
		}

		// 7.8)
		if expandedValue == nil {
			continue
		}
		// 7.9)
		if termCtx.HasContainerMapping(key, "@list") {
			expandedValueMap, isMap := expandedValue.(map[string]interface{})
			_, containsList := expandedValueMap["@list"]
			if !isMap || !containsList {
				newExpandedValue := make(map[string]interface{}, 1)
				_, isList := expandedValue.([]interface{})
				if !isList {
					newExpandedValue["@list"] = []interface{}{expandedValue}
				} else {
					newExpandedValue["@list"] = expandedValue
				}
				expandedValue = newExpandedValue
			}
		}

		isContainerGraph := termCtx.HasContainerMapping(key, "@graph")
		isContainerID := termCtx.HasContainerMapping(key, "@id")
		isContainerIndex := termCtx.HasContainerMapping(key, "@index")
		if isContainerGraph && !isContainerID && !isContainerIndex {
			evList := Arrayify(expandedValue)
			rVal := make([]interface{}, 0)
			for _, ev := range evList {
				ev = map[string]interface{}{
					"@graph": Arrayify(ev),
				}
				rVal = append(rVal, ev)
			}
			expandedValue = rVal
		}

		// 7.10)
		if termCtx.IsReverseProperty(key) {
			var reverseMap map[string]interface{}
			if reverseValue, containsReverse := resultMap["@reverse"]; containsReverse {
				// 7.10.2)
				reverseMap = reverseValue.(map[string]interface{})
			} else {
				// 7.10.1)
				reverseMap = make(map[string]interface{})
				resultMap["@reverse"] = reverseMap
			}

			// 7.10.3)
			expandedValueList, isList := expandedValue.([]interface{})
			if !isList {
				expandedValueList = []interface{}{expandedValue}
				expandedValue = expandedValueList
			}
			// 7.10.4)
			for _, item := range expandedValueList {

				// 7.10.4.2)
				var expandedPropertyList []interface{}
				expandedPropertyValue, containsExpandedProperty := reverseMap[expandedProperty]
				if containsExpandedProperty {
					expandedPropertyList = expandedPropertyValue.([]interface{})
				} else {
					expandedPropertyList = make([]interface{}, 0)
				}

				switch v := item.(type) {
				case map[string]interface{}:
					// 7.10.4.1)
					_, containsValue := v["@value"]
					_, containsList := v["@list"]
					if containsValue || containsList {
						return NewJsonLdError(InvalidReversePropertyValue, nil)
					}
					expandedPropertyList = append(expandedPropertyList, v)
				case []interface{}:
					// 7.10.4.3)
					expandedPropertyList = append(expandedPropertyList, v...)
				default:
					expandedPropertyList = append(expandedPropertyList, v)
				}
				reverseMap[expandedProperty] = expandedPropertyList
			}
		} else { // 7.11)
			// 7.11.1)
			var expandedPropertyList []interface{}
			expandedPropertyValue, containsExpandedProperty := resultMap[expandedProperty]
			if containsExpandedProperty {
				expandedPropertyList = expandedPropertyValue.([]interface{})
			} else {
				expandedPropertyList = make([]interface{}, 0)
				resultMap[expandedProperty] = expandedPropertyList
			}
			// 7.11.2)
			if expandedValueList, isList := expandedValue.([]interface{}); isList {
				expandedPropertyList = append(expandedPropertyList, expandedValueList...)
			} else {
				expandedPropertyList = append(expandedPropertyList, expandedValue)
			}
			resultMap[expandedProperty] = expandedPropertyList
		}
	}

	// expand each nested key
	for _, n := range nests {
		for _, nv := range Arrayify(elem[n]) {
			nvMap, isMap := nv.(map[string]interface{})
			hasValues := false
			if isMap {
				for k := range nvMap {
					expanded, _ := activeCtx.ExpandIri(k, false, true, nil, nil)
					if expanded == "@value" {
						hasValues = true
						break
					}
				}
			}
			if !isMap || hasValues {
				return NewJsonLdError(InvalidNestValue, "nested value must be a node object")
			}
			err := api.expandObject(activeCtx, activeProperty, expandedActiveProperty, nv.(map[string]interface{}), resultMap, typeKey, opts, typeScopedContext, frameExpansion)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (api *JsonLdApi) expandIndexMap(activeCtx *Context, activeProperty string, value map[string]interface{}, indexKey string, asGraph bool, propertyIndex string, opts *JsonLdOptions) (interface{}, error) {
	// 7.6.1)
	var expandedValueList []interface{}
	// 7.6.2)
	for _, key := range GetOrderedKeys(value) {
		indexValue := value[key]

		indexCtx := activeCtx
		// if indexKey is @type, there may be a context defined for it
		if td := activeCtx.GetTermDefinition(key); indexKey == "@type" && td != nil && td.hasContext {
			newCtx, err := activeCtx.Parse(td.context)
			if err != nil {
				return nil, err
			}
			indexCtx = newCtx
		}

		// 7.6.2.1)
		indexValue = Arrayify(indexValue)

		// 7.6.2.2)
		indexValue, err := api.Expand(indexCtx, activeProperty, indexValue, opts, true, nil)
		if err != nil {
			return nil, err
		}

		// expand for @type, but also for @none
		var expandedKey interface{}
		if propertyIndex != "" {
			if key == "@none" {
				expandedKey = "@none"
			} else {
				expandedKeyVal, err := indexCtx.ExpandValue(indexKey, key)
				if err != nil {
					return nil, err
				}
				expandedKey = expandedKeyVal
			}
		} else {
			expandedKey, err = indexCtx.ExpandIri(key, false, true, nil, nil)
			if err != nil {
				return nil, err
			}
		}

		if indexKey == "@id" {
			// expand document relative
			key, err = indexCtx.ExpandIri(key, true, false, nil, nil)
			if err != nil {
				return nil, err
			}
		} else if indexKey == "@type" {
			key = expandedKey.(string)
		}

		// 7.6.2.3)
		for _, itemValue := range indexValue.([]interface{}) {
			if asGraph && !IsGraph(itemValue) {
				itemValue = map[string]interface{}{
					"@graph": Arrayify(itemValue),
				}
			}
			item := itemValue.(map[string]interface{})
			if indexKey == "@type" {
				if expandedKey == "@none" {
					// ignore @none
				} else if types, hasType := item["@type"]; hasType {
					switch v := types.(type) {
					case string:
						item["@type"] = []interface{}{key, v}
					case []interface{}:
						item["@type"] = append([]interface{}{key}, v...)
					}
				} else {
					item["@type"] = []interface{}{key}
				}
			} else if IsValue(item) && indexKey != "@language" && indexKey != "@type" && indexKey != "@index" {
				return nil, NewJsonLdError(InvalidValueObject,
					fmt.Sprintf("Attempt to add illegal key to value object: %s", indexKey))
			} else if propertyIndex != "" {
				// index is a property to be expanded, and values interpreted for that property
				if expandedKey != "@none" {
					// expand key as a value
					AddValue(item, propertyIndex, expandedKey, true, false, true, true)
				}

			} else if _, containsKey := item[indexKey]; !containsKey && expandedKey != "@none" {
				// 7.6.2.3.1)
				item[indexKey] = key
			}

			// 7.6.2.3.2)
			expandedValueList = append(expandedValueList, item)
		}
	}
	return expandedValueList, nil
}
