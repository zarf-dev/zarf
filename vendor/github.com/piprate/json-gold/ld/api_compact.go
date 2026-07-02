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
	"sort"
)

// Compact operation compacts the given input using the context
// according to the steps in the Compaction Algorithm:
//
// http://www.w3.org/TR/json-ld-api/#compaction-algorithm
//
// Returns the compacted JSON-LD object.
// Returns an error if there was an error during compaction.
func (api *JsonLdApi) Compact(activeCtx *Context, activeProperty string, element interface{},
	compactArrays bool) (interface{}, error) {

	if elementList, isList := element.([]interface{}); isList {
		result := make([]interface{}, 0)
		for _, item := range elementList {
			compactedItem, err := api.Compact(activeCtx, activeProperty, item, compactArrays)
			if err != nil {
				return nil, err
			}
			if compactedItem != nil {
				result = append(result, compactedItem)
			}
		}

		if compactArrays && len(result) == 1 && len(activeCtx.GetContainer(activeProperty)) == 0 {
			return result[0], nil
		}

		return result, nil
	}

	// use any scoped context on activeProperty
	if td := activeCtx.GetTermDefinition(activeProperty); td != nil && td.hasContext {
		newCtx, err := activeCtx.parse(td.context, make([]string, 0), false, true, false, true)
		if err != nil {
			return nil, err
		}
		activeCtx = newCtx
	}

	if elem, isMap := element.(map[string]interface{}); isMap {

		// do value compaction on @values and subject references
		if IsValue(elem) || IsSubjectReference(elem) {
			compactedValue, err := activeCtx.CompactValue(activeProperty, elem)
			if err != nil {
				return nil, err
			}

			propType := ""
			if td := activeCtx.GetTermDefinition(activeProperty); td != nil {
				propType = td.typ
			}
			if _, isMap := compactedValue.(map[string]interface{}); !isMap || propType == "@json" {
				return compactedValue, nil
			}
		}

		// if expanded property is @list and we're contained within a list container,
		// recursively compact this item to an array
		if list, containsList := elem["@list"]; containsList {
			if isListContainer := activeCtx.HasContainerMapping(activeProperty, "@list"); isListContainer {
				return api.Compact(activeCtx, activeProperty, list, compactArrays)
			}
		}

		insideReverse := activeProperty == "@reverse"

		result := make(map[string]interface{})

		// original context before applying property-scoped and local contexts
		inputCtx := activeCtx

		// revert to previous context, if there is one,
		// and element is not a value object or a node reference
		if !IsValue(elem) && !IsSubjectReference(elem) {
			activeCtx = activeCtx.RevertToPreviousContext()
		}

		// apply property-scoped context after reverting term-scoped context
		if td := inputCtx.GetTermDefinition(activeProperty); td != nil && td.context != nil {
			newCtx, err := activeCtx.parse(td.context, nil, false, true, false, true)
			if err != nil {
				return nil, err
			}
			activeCtx = newCtx
		}

		// apply any context defined on an alias of @type
		// if key is @type and any compacted value is a term having a local
		// context, overlay that context
		if typeVal, hasType := elem["@type"]; hasType {
			// set scoped contexts from @type
			types := make([]string, 0)
			typeContext := activeCtx
			for _, t := range Arrayify(typeVal) {
				if typeStr, isString := t.(string); isString {
					compactedType, err := typeContext.CompactIri(typeStr, nil, true, false)
					if err != nil {
						return nil, err
					}
					types = append(types, compactedType)
				}
			}
			// process in lexicographical order, see https://github.com/json-ld/json-ld.org/issues/616
			sort.Strings(types)
			for _, tt := range types {
				if td := inputCtx.GetTermDefinition(tt); td != nil && td.hasContext {
					newCtx, err := activeCtx.parse(td.context, nil, false, false, false, false)
					if err != nil {
						return nil, err
					}
					activeCtx = newCtx
				}
			}
		}

		// recursively process element keys in order
		for _, expandedProperty := range GetOrderedKeys(elem) {
			expandedValue := elem[expandedProperty]

			if expandedProperty == "@id" {

				alias, err := activeCtx.CompactIri(expandedProperty, nil, true, false)
				if err != nil {
					return nil, err
				}

				var compactedValue interface{}

				compactedValues := make([]interface{}, 0)

				for _, v := range Arrayify(expandedValue) {
					cv, err := activeCtx.CompactIri(v.(string), nil, false, false)
					if err != nil {
						return nil, err
					}
					compactedValues = append(compactedValues, cv)
				}

				if len(compactedValues) == 1 {
					compactedValue = compactedValues[0]
				} else {
					compactedValue = compactedValues
				}

				result[alias] = compactedValue

				continue
			}

			if expandedProperty == "@type" {
				alias, err := activeCtx.CompactIri(expandedProperty, nil, true, false)
				if err != nil {
					return nil, err
				}

				var compactedValue interface{}

				compactedValues := make([]interface{}, 0)

				for _, v := range Arrayify(expandedValue) {
					cv, err := inputCtx.CompactIri(v.(string), nil, true, false)
					if err != nil {
						return nil, err
					}
					compactedValues = append(compactedValues, cv)
				}

				container := activeCtx.GetContainer(alias)
				isTypeContainer := expandedProperty == "@type" && (len(container) > 0 && container[0] == "@set")
				if len(compactedValues) == 1 && (!activeCtx.processingMode(1.1) || !isTypeContainer) {
					compactedValue = compactedValues[0]
				} else {
					compactedValue = compactedValues
				}

				// TODO: review and simplify, see JS and Ruby implementations
				compValArray, isArray := compactedValue.([]interface{})
				AddValue(result, alias, compactedValue, isArray && (len(compValArray) == 0 || isTypeContainer), false, true, false)

				continue
			}

			if expandedProperty == "@reverse" {

				compactedObject, _ := api.Compact(activeCtx, "@reverse", expandedValue, compactArrays)
				compactedValue := compactedObject.(map[string]interface{})

				for _, property := range GetKeys(compactedValue) {
					value := compactedValue[property]

					if activeCtx.IsReverseProperty(property) {
						useArray := activeCtx.HasContainerMapping(property, "@set") || !compactArrays

						AddValue(result, property, value, useArray, false, true, false)

						delete(compactedValue, property)
					}

				}

				if len(compactedValue) > 0 {
					alias, err := activeCtx.CompactIri("@reverse", nil, false, false)
					if err != nil {
						return nil, err
					}
					AddValue(result, alias, compactedValue, false, false, true, false)
				}

				continue
			}

			if expandedProperty == "@preserve" {
				// compact using activeProperty
				compactedValue, _ := api.Compact(activeCtx, activeProperty, expandedValue, compactArrays)
				if cva, isArray := compactedValue.([]interface{}); !(isArray && len(cva) == 0) {
					AddValue(result, expandedProperty, compactedValue, false, false, true, false)
				}
				continue
			}

			if expandedProperty == "@index" && activeCtx.HasContainerMapping(activeProperty, "@index") {
				continue
			} else if expandedProperty == "@index" || expandedProperty == "@value" || expandedProperty == "@language" ||
				expandedProperty == "@direction" {
				alias, err := activeCtx.CompactIri(expandedProperty, nil, false, false)
				if err != nil {
					return nil, err
				}
				AddValue(result, alias, expandedValue, false, false, true, false)
				continue
			}

			// skip array processing for keywords that aren't @graph or @list
			if expandedProperty != "@graph" && expandedProperty != "@list" && IsKeyword(expandedProperty) {
				alias, err := activeCtx.CompactIri(expandedProperty, nil, false, false)
				if err != nil {
					return nil, err
				}
				AddValue(result, alias, expandedValue, false, false, true, false)
				continue
			}

			// NOTE: expanded value must be an array due to expansion algorithm.

			expandedValueList, isList := expandedValue.([]interface{})
			if isList && len(expandedValueList) == 0 {

				// preserve empty arrays

				itemActiveProperty, err := activeCtx.CompactIri(expandedProperty, expandedValue, true, insideReverse)
				if err != nil {
					return nil, err
				}

				nestResult := result
				if td := activeCtx.GetTermDefinition(itemActiveProperty); td != nil && td.nest != "" {
					nestProperty := td.nest
					if err := api.checkNestProperty(activeCtx, nestProperty); err != nil {
						return nil, err
					}
					if _, isMap := result[nestProperty].(map[string]interface{}); !isMap {
						result[nestProperty] = make(map[string]interface{})
					}
					nestResult = result[nestProperty].(map[string]interface{})
				}

				AddValue(nestResult, itemActiveProperty, make([]interface{}, 0), true, false, true, false)
			}

			for _, expandedItem := range expandedValueList {
				itemActiveProperty, err := activeCtx.CompactIri(expandedProperty, expandedItem, true, insideReverse)
				if err != nil {
					return nil, err
				}
				isListContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@list")
				isGraphContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@graph")
				isSetContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@set")
				isLanguageContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@language")
				isIndexContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@index")
				isIDContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@id")
				isTypeContainer := activeCtx.HasContainerMapping(itemActiveProperty, "@type")

				// if itemActiveProperty is a @nest property, add values to nestResult, otherwise result
				nestResult := result

				if td := activeCtx.GetTermDefinition(itemActiveProperty); td != nil && td.nest != "" {
					nestProperty := td.nest
					if err := api.checkNestProperty(activeCtx, nestProperty); err != nil {
						return nil, err
					}
					if _, isMap := result[nestProperty].(map[string]interface{}); !isMap {
						result[nestProperty] = make(map[string]interface{})
					}
					nestResult = result[nestProperty].(map[string]interface{})
				}

				// get @list value if appropriate
				expandedItemMap, isMap := expandedItem.(map[string]interface{})
				isGraph := IsGraph(expandedItemMap)
				list, containsList := expandedItemMap["@list"]
				isList := isMap && containsList
				var inner interface{}

				if isList {
					inner = list
				} else if isGraph {
					inner = expandedItemMap["@graph"]
				}

				var elementToCompact interface{}
				if isList || isGraph {
					elementToCompact = inner
				} else {
					elementToCompact = expandedItem
				}

				// recursively compact expanded item
				compactedItem, err := api.Compact(activeCtx, itemActiveProperty, elementToCompact, compactArrays)
				if err != nil {
					return nil, err
				}

				if isList {
					compactedItem = Arrayify(compactedItem)

					if !isListContainer {

						listAlias, err := activeCtx.CompactIri("@list", nil, false, false)
						if err != nil {
							return nil, err
						}
						wrapper := map[string]interface{}{
							listAlias: compactedItem,
						}
						compactedItem = wrapper

						if indexVal, containsIndex := expandedItemMap["@index"]; containsIndex {
							indexAlias, err := activeCtx.CompactIri("@index", nil, false, false)
							if err != nil {
								return nil, err
							}
							wrapper[indexAlias] = indexVal
						}
					} else {
						AddValue(nestResult, itemActiveProperty, compactedItem, true, true, true, false)
						continue
					}
				}

				// graph object compaction
				if isGraph {
					asArray := !compactArrays || isSetContainer
					if isGraphContainer && (isIDContainer || isIndexContainer && IsSimpleGraph(expandedItemMap)) {
						var mapObject map[string]interface{}
						if v, present := nestResult[itemActiveProperty]; present {
							mapObject = v.(map[string]interface{})
						} else {
							mapObject = make(map[string]interface{})
							nestResult[itemActiveProperty] = mapObject
						}

						// index on @id or @index or alias of @none
						k := "@index"
						if isIDContainer {
							k = "@id"
						}
						var mapKey string
						if v, found := expandedItemMap[k]; found {
							mapKey = v.(string)
						} else {
							mapKey, err = activeCtx.CompactIri("@none", nil, false, false)
							if err != nil {
								return nil, err
							}
						}

						// add compactedItem to map, using value of "@id" or a new blank node identifier
						AddValue(mapObject, mapKey, compactedItem, asArray, false, true, false)
					} else if isGraphContainer && IsSimpleGraph(expandedItemMap) {

						// container includes @graph but not @id or @index and value is a
						// simple graph object add compact value
						compactedItemArray, isArray := compactedItem.([]interface{})
						if isArray && len(compactedItemArray) > 1 {
							// multiple objects in the same graph can't be represented directly,
							// as they would be interpreted as two different graphs.
							// Need to wrap in @included.
							includedKey, err := activeCtx.CompactIri("@included", nil, true, false)
							if err != nil {
								return nil, err
							}
							compactedItem = map[string]interface{}{
								includedKey: compactedItem,
							}
						}

						AddValue(nestResult, itemActiveProperty, compactedItem, asArray, false, true, false)
					} else {
						// wrap using @graph alias, remove array if only one item and compactArrays not set
						compactedItemArray, isArray := compactedItem.([]interface{})
						if isArray && len(compactedItemArray) == 1 && compactArrays {
							compactedItem = compactedItemArray[0]
						}
						graphAlias, err := activeCtx.CompactIri("@graph", nil, false, false)
						if err != nil {
							return nil, err
						}
						compactedItemMap := map[string]interface{}{
							graphAlias: compactedItem,
						}
						compactedItem = compactedItemMap

						// include @id from expanded graph, if any
						if val, hasID := expandedItemMap["@id"]; hasID {
							idAlias, err := activeCtx.CompactIri("@id", nil, false, false)
							if err != nil {
								return nil, err
							}
							compactedItemMap[idAlias] = val
						}

						// include @index from expanded graph, if any
						if val, hasIndex := expandedItemMap["@index"]; hasIndex {
							indexAlias, err := activeCtx.CompactIri("@index", nil, false, false)
							if err != nil {
								return nil, err
							}
							compactedItemMap[indexAlias] = val
						}

						AddValue(nestResult, itemActiveProperty, compactedItem, asArray, false, true, false)
					}
				} else if isLanguageContainer || isIndexContainer || isIDContainer || isTypeContainer {

					var mapObject map[string]interface{}
					if v, present := nestResult[itemActiveProperty]; present {
						mapObject = v.(map[string]interface{})
					} else {
						mapObject = make(map[string]interface{})
						nestResult[itemActiveProperty] = mapObject
					}

					var mapKey string

					if isLanguageContainer {
						compactedItemMap, isMap := compactedItem.(map[string]interface{})
						compactedItemValue, containsValue := compactedItemMap["@value"]
						if isLanguageContainer && isMap && containsValue {
							compactedItem = compactedItemValue
						}
						if v, found := expandedItemMap["@language"]; found {
							mapKey = v.(string)
						}
					} else if isIndexContainer {

						indexKey := "@index"
						if td := activeCtx.GetTermDefinition(itemActiveProperty); td != nil && td.index != "" {
							indexKey = td.index
						}

						containerKey, err := activeCtx.CompactIri(indexKey, nil, true, false)
						if err != nil {
							return nil, err
						}

						if indexKey == "@index" {
							mapKey, _ = expandedItemMap["@index"].(string)
							if compactedItemMap, isMap := compactedItem.(map[string]interface{}); isMap {
								delete(compactedItemMap, containerKey)
							}
						} else {
							var propsArray []interface{}
							compactedItemMap, isMap := compactedItem.(map[string]interface{})
							if isMap {
								props, found := compactedItemMap[indexKey]
								if found {
									propsArray = Arrayify(props)
								} else {
									propsArray = make([]interface{}, 0)
								}
							}

							var mapKeyVal interface{}
							var others []interface{}
							if len(propsArray) > 0 {
								mapKeyVal = propsArray[0]
								others = propsArray[1:]
							}
							var isString bool
							if mapKey, isString = mapKeyVal.(string); !isString {
								mapKey = ""
							} else {
								switch len(others) {
								case 0:
									delete(compactedItemMap, indexKey)
								case 1:
									compactedItemMap[indexKey] = others[0]
								default:
									compactedItemMap[indexKey] = others
								}
							}
						}
					} else if isIDContainer {
						idKey, err := activeCtx.CompactIri("@id", nil, false, false)
						if err != nil {
							return nil, err
						}
						compactedItemMap := compactedItem.(map[string]interface{})
						if compactedItemValue, containsValue := compactedItemMap[idKey]; containsValue {
							mapKey = compactedItemValue.(string)
							delete(compactedItemMap, idKey)
						} else {
							mapKey = ""
						}
					} else if isTypeContainer {
						typeKey, err := activeCtx.CompactIri("@type", nil, false, false)
						if err != nil {
							return nil, err
						}

						compactedItemMap := compactedItem.(map[string]interface{})
						var types []interface{}
						if compactedItemValue, containsValue := compactedItemMap[typeKey]; containsValue {
							var isArray bool
							types, isArray = compactedItemValue.([]interface{})
							if !isArray {
								types = []interface{}{compactedItemValue}
							}

							delete(compactedItemMap, typeKey)
							if len(types) > 0 {
								mapKey = types[0].(string)
								types = types[1:]
							}
						} else {
							types = make([]interface{}, 0)
						}

						// if compactedItem contains a single entry whose key maps to @id, re-compact without @type
						if len(compactedItemMap) == 1 {
							if idVal, hasID := expandedItemMap["@id"]; hasID {
								compactedItem, err = api.Compact(activeCtx, itemActiveProperty,
									map[string]interface{}{
										"@id": idVal,
									}, compactArrays)
								if err != nil {
									return nil, err
								}
							}
						}

						if len(types) > 0 {
							AddValue(compactedItemMap, typeKey, types, false, false, false, false)
						}
					}

					if mapKey == "" {
						mapKey, err = activeCtx.CompactIri("@none", nil, true, false)
						if err != nil {
							return nil, err
						}
					}

					AddValue(mapObject, mapKey, compactedItem, isSetContainer, false, true, false)
				} else {
					compactedItemArray, isArray := compactedItem.([]interface{})

					asArray := !compactArrays || isSetContainer || isListContainer ||
						(isArray && len(compactedItemArray) == 0) || expandedProperty == "@list" ||
						expandedProperty == "@graph"
					AddValue(nestResult, itemActiveProperty, compactedItem, asArray, false, true, false)
				}
			}
		}

		return result, nil
	}

	return element, nil
}

// checkNestProperty ensures that the value of `@nest` in the term definition must
// either be "@nest", or a term which resolves to "@nest".
func (api *JsonLdApi) checkNestProperty(activeCtx *Context, nestProperty string) error {
	if v, _ := activeCtx.ExpandIri(nestProperty, false, true, nil, nil); v != "@nest" {
		return NewJsonLdError(InvalidNestValue, "nested property must have an @nest value resolving to @nest")
	}
	return nil
}
