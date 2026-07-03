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
	"fmt"
	"strings"
)

// GenerateNodeMap recursively flattens the subjects in the given JSON-LD expanded
// input into a node map.
func (api *JsonLdApi) GenerateNodeMap(element interface{}, graphMap map[string]interface{}, activeGraph string,
	issuer *IdentifierIssuer, activeSubject interface{}, activeProperty string, list map[string]interface{}) (map[string]interface{}, error) {

	// recurse through array
	if elementList, isList := element.([]interface{}); isList {
		// if element is an array, process each entry in element recursively by passing item for element,
		// node map, active graph, active subject, active property, and list.
		for _, item := range elementList {
			var err error
			list, err = api.GenerateNodeMap(item, graphMap, activeGraph, issuer, activeSubject, activeProperty, list)
			if err != nil {
				return nil, err
			}
		}
		return list, nil
	}

	// add non-object to list
	elem, isMap := element.(map[string]interface{})
	if !isMap {
		return nil, fmt.Errorf("expected map or list to GenerateNodeMap, got %T", element)
	}

	var graph map[string]interface{}
	if graphVal, found := graphMap[activeGraph]; found {
		graph = graphVal.(map[string]interface{})
	} else {
		graph = make(map[string]interface{})
		graphMap[activeGraph] = graph
	}

	var subjectNode interface{}
	if activeSubject == nil {
		subjectNode = graph
	} else if _, isString := activeSubject.(string); isString {
		subjectNode = graph[activeSubject.(string)]
	} else {
		subjectNode = make(map[string]interface{})
	}

	// transform bnode types
	if typeVal, hasType := elem["@type"]; hasType {
		types := Arrayify(typeVal)
		newTypes := make([]interface{}, len(types))
		for i, t := range types {
			typeStr := t.(string)
			if strings.HasPrefix(typeStr, "_:") { // use IsBlankNodeValue()
				typeStr = issuer.GetId(typeStr)
			}
			newTypes[i] = typeStr
		}
		if IsValue(element) {
			elem["@type"] = newTypes[0]
		} else {
			elem["@type"] = newTypes
		}
	}

	if IsValue(element) {
		if list == nil {
			AddValue(subjectNode, activeProperty, element, true, false, false, false)
		} else {
			list["@list"] = append(list["@list"].([]interface{}), element)
		}
		return list, nil
	} else if IsList(element) {
		result := map[string]interface{}{
			"@list": []interface{}{},
		}
		var err error
		result, err = api.GenerateNodeMap(elem["@list"], graphMap, activeGraph, issuer, activeSubject, activeProperty, result)
		if err != nil {
			return nil, err
		}
		if list == nil {
			AddValue(subjectNode, activeProperty, result, true, false, false, false)
		} else {
			list["@list"] = append(list["@list"].([]interface{}), result)
		}
		return list, nil
	}

	// element is a node object

	id := elem["@id"]
	if id == nil {
		id = issuer.GetId("")
	} else if strings.HasPrefix(id.(string), "_:") {
		id = issuer.GetId(id.(string))
	}

	nodeVal, found := graph[id.(string)]
	if !found {
		nodeVal = map[string]interface{}{
			"@id": id,
		}
		graph[id.(string)] = nodeVal
	}
	node := nodeVal.(map[string]interface{})

	if _, isMap := activeSubject.(map[string]interface{}); isMap {
		// if subject is a hash, then we're processing a reverse-property relationship.
		AddValue(node, activeProperty, activeSubject, true, false, false, false)
	} else if activeProperty != "" {
		ref := map[string]interface{}{
			"@id": id,
		}
		if list == nil {
			AddValue(subjectNode, activeProperty, ref, true, false, false, false)
		} else {
			list["@list"] = append(list["@list"].([]interface{}), ref)
		}
	}

	if typeVal, hasType := elem["@type"]; hasType {
		AddValue(node, "@type", typeVal, true, false, false, false)
	}

	if elemIdx, hasIndex := elem["@index"]; hasIndex {
		if nodeIdx, found := node["@index"]; found && nodeIdx != elemIdx {
			return nil, NewJsonLdError(ConflictingIndexes, "conflicting @index property detected")
		}
		node["@index"] = elemIdx
	}

	// handle reverse properties
	if reverseVal, hasReverse := elem["@reverse"]; hasReverse {
		referencedNode := map[string]interface{}{
			"@id": id,
		}
		reverseMap := reverseVal.(map[string]interface{})
		for reverseProperty, values := range reverseMap {
			for _, v := range values.([]interface{}) {
				_, err := api.GenerateNodeMap(v, graphMap, activeGraph, issuer, referencedNode, reverseProperty, nil)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if graphVal, hasGraph := elem["@graph"]; hasGraph {
		_, err := api.GenerateNodeMap(graphVal, graphMap, id.(string), issuer, "", "", nil)
		if err != nil {
			return nil, err
		}
	}

	if includedVal, hasIncluded := elem["@included"]; hasIncluded {
		_, err := api.GenerateNodeMap(includedVal, graphMap, activeGraph, issuer, "", "", nil)
		if err != nil {
			return nil, err
		}
	}

	for _, property := range GetOrderedKeys(elem) {
		if property == "@id" || property == "@type" || property == "@index" || property == "@reverse" ||
			property == "@graph" || property == "@included" {
			// already processed
			continue
		}

		value := elem[property]

		// if property is a bnode, assign it a new id
		if strings.HasPrefix(property, "_:") {
			property = issuer.GetId(property)
		}

		if _, found := node[property]; !found {
			node[property] = []interface{}{}
		}
		if _, err := api.GenerateNodeMap(value, graphMap, activeGraph, issuer, id.(string), property, nil); err != nil {
			return nil, err
		}
	}

	return list, nil
}
