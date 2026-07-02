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
	"sort"
)

// UsagesNode is a helper class for node usages
type UsagesNode struct {
	node     *NodeMapNode
	property string
	value    map[string]interface{}
}

// NewUsagesNode creates a new instance of UsagesNode
func NewUsagesNode(node *NodeMapNode, property string, value map[string]interface{}) *UsagesNode {
	return &UsagesNode{
		node:     node,
		property: property,
		value:    value,
	}
}

// NodeMapNode
type NodeMapNode struct {
	Values map[string]interface{}
	usages []*UsagesNode
}

// NewNodeMapNode creates a new instance of NodeMapNode.
func NewNodeMapNode(id string) *NodeMapNode {
	return &NodeMapNode{
		Values: map[string]interface{}{"@id": id},
		usages: make([]*UsagesNode, 0),
	}
}

// IsReferencedOnce helps to solve https://github.com/json-ld/json-ld.org/issues/357
// by identifying nodes with just one reference.
func IsReferencedOnce(node *NodeMapNode, referencedOnce map[string]*UsagesNode) bool {
	referencedOnceUsage, present := referencedOnce[node.Values["@id"].(string)]
	return present && referencedOnceUsage != nil
}

// IsWellFormedListNode is a helper function for 4.3.3
func (nmn *NodeMapNode) IsWellFormedListNode() bool {
	keys := 0
	v, containsRdfFirst := nmn.Values[RDFFirst]
	if containsRdfFirst {
		keys++
		vList, isList := v.([]interface{})
		if !(isList && len(vList) == 1) {
			return false
		}
	}
	v, containsRdfRest := nmn.Values[RDFRest]
	if containsRdfRest {
		keys++
		vList, isList := v.([]interface{})
		if !(isList && len(vList) == 1) {
			return false
		}
	}
	v, containsType := nmn.Values["@type"]
	if containsType {
		keys++
		vList, isList := v.([]interface{})
		if !(isList && len(vList) == 1 && vList[0] == RDFList) {
			return false
		}
	}
	// TODO: SPEC: 4.3.3 has no mention of @id
	_, containsID := nmn.Values["@id"]
	if containsID {
		keys++
	}
	if keys < len(nmn.Values) {
		return false
	}
	return true
}

// Serialize returns this node without the usages variable
func (nmn *NodeMapNode) Serialize() map[string]interface{} {
	rval := make(map[string]interface{}, len(nmn.Values))
	for k, v := range nmn.Values {
		rval[k] = v
	}
	return rval
}

// FromRDF converts RDF statements into JSON-LD.
// Returns a list of JSON-LD objects found in the given dataset.
func (api *JsonLdApi) FromRDF(dataset *RDFDataset, opts *JsonLdOptions) ([]interface{}, error) {
	// 1)
	defaultGraph := make(map[string]*NodeMapNode)
	// 2)
	graphMap := make(map[string]map[string]*NodeMapNode)
	graphMap["@default"] = defaultGraph
	referencedOnceMap := make(map[string]*UsagesNode)

	// 3/3.1)
	for name, graph := range dataset.Graphs {
		// 3.2+3.4)
		nodeMap, present := graphMap[name]
		if !present {
			nodeMap = make(map[string]*NodeMapNode)
			graphMap[name] = nodeMap
		}

		// 3.3)
		if _, present := defaultGraph[name]; name != "@default" && !present {
			defaultGraph[name] = NewNodeMapNode(name)
		}

		// 3.5)
		for _, triple := range graph {
			subject := triple.Subject.GetValue()
			predicate := triple.Predicate.GetValue()
			object := triple.Object

			// 3.5.1+3.5.2)
			node, present := nodeMap[subject]
			if !present {
				node = NewNodeMapNode(subject)
				nodeMap[subject] = node
			}

			// 3.5.3)
			_, containsObject := nodeMap[object.GetValue()]
			if (IsIRI(object) || IsBlankNode(object)) && !containsObject {
				nodeMap[object.GetValue()] = NewNodeMapNode(object.GetValue())
			}

			// 3.5.4)
			if predicate == RDFType && (IsIRI(object) || IsBlankNode(object)) && !opts.UseRdfType {
				MergeValue(node.Values, "@type", object.GetValue())
				continue
			}

			// 3.5.5)
			value, err := RdfToObject(object, opts.UseNativeTypes)
			if err != nil {
				return nil, err
			}

			// 3.5.6+7)
			MergeValue(node.Values, predicate, value)

			// 3.5.8)
			if IsBlankNode(object) || IsIRI(object) {
				// track rdf:nil uniquely per graph
				if object.GetValue() == RDFNil {
					// 3.5.8.1-3)
					n := nodeMap[object.GetValue()]
					n.usages = append(n.usages, NewUsagesNode(node, predicate, value))
				} else if _, present := referencedOnceMap[object.GetValue()]; present {
					referencedOnceMap[object.GetValue()] = nil
				} else {
					// track single reference
					referencedOnceMap[object.GetValue()] = NewUsagesNode(node, predicate, value)
				}
			}
		}
	}

	// 4)
	for _, graph := range graphMap {
		// 4.1), 4.2)
		nilNode, present := graph[RDFNil]
		if !present {
			continue
		}
		// 4.3)
		for _, usage := range nilNode.usages {
			// 4.3.1)
			node := usage.node
			property := usage.property
			head := usage.value
			// 4.3.2)
			list := make([]interface{}, 0)
			listNodes := make([]string, 0)
			// 4.3.3)
			for property == RDFRest && IsReferencedOnce(node, referencedOnceMap) && node.IsWellFormedListNode() {
				// 4.3.3.1)
				list = append(list, node.Values[RDFFirst].([]interface{})[0])
				// 4.3.3.2)
				listNodes = append(listNodes, node.Values["@id"].(string))
				// 4.3.3.3)
				nodeUsage := referencedOnceMap[node.Values["@id"].(string)]
				// 4.3.3.4)
				node = nodeUsage.node
				property = nodeUsage.property
				head = nodeUsage.value
				// if node is not a blank node, then list head found
				if !IsBlankNodeValue(node.Values) {
					break
				}
			}

			// 4.3.5)
			delete(head, "@id")
			// 4.3.6)
			// reverse the list
			for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
				list[i], list[j] = list[j], list[i]
			}
			// 4.3.7)
			head["@list"] = list
			// 4.3.8)
			for _, nodeID := range listNodes {
				delete(graph, nodeID)
			}
		}
	}

	// 5)
	result := make([]interface{}, 0)

	// 6)
	ids := make([]string, 0)
	for k := range defaultGraph {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	for _, subject := range ids {
		node := defaultGraph[subject]
		// 6.1)
		subjectMap, containsSubj := graphMap[subject]
		if containsSubj {
			// 6.1.1)
			graph := make([]interface{}, 0)
			// 6.1.2)
			keys := make([]string, 0)
			for k := range subjectMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, s := range keys {
				n := subjectMap[s]
				_, containsID := n.Values["@id"]
				if len(n.Values) == 1 && containsID {
					continue
				}
				graph = append(graph, n.Serialize())
			}
			node.Values["@graph"] = graph
		}
		// 6.2)
		_, containsID := node.Values["@id"]
		if len(node.Values) == 1 && containsID {
			continue
		}
		result = append(result, node.Serialize())
	}

	return result, nil
}
