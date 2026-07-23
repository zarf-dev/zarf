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
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/piprate/json-gold/ld/internal/jsoncanonicalizer"
)

// Node is the value of a subject, predicate or object
// i.e. a IRI reference, blank node or literal.
type Node interface {
	// GetValue returns the node's value.
	GetValue() string

	// Equal returns true if this node is equal to the given node.
	Equal(n Node) bool
}

// Literal represents a literal value.
type Literal struct {
	Value    string
	Datatype string
	Language string
}

// NewLiteral creates a new instance of Literal.
func NewLiteral(value string, datatype string, language string) Literal {
	l := Literal{
		Value:    value,
		Language: language,
	}

	if datatype != "" {
		l.Datatype = datatype
	} else {
		l.Datatype = XSDString
	}

	return l
}

// GetValue returns the node's value.
func (l Literal) GetValue() string {
	return l.Value
}

// Equal returns true if this node is equal to the given node.
func (l Literal) Equal(n Node) bool {
	ol, ok := n.(*Literal)
	if !ok {
		return false
	}

	if l.Value != ol.Value {
		return false
	}

	if l.Language != ol.Language {
		return false
	}

	if l.Datatype != ol.Datatype {
		return false
	}

	return true
}

// IRI represents an IRI value.
type IRI struct {
	Value string
}

// NewIRI creates a new instance of IRI.
func NewIRI(iri string) IRI {
	i := IRI{
		Value: iri,
	}

	return i
}

// GetValue returns the node's value.
func (iri IRI) GetValue() string {
	return iri.Value
}

// Equal returns true if this node is equal to the given node.
func (iri IRI) Equal(n Node) bool {
	if oiri, ok := n.(*IRI); ok {
		return iri.Value == oiri.Value
	}

	return false
}

// BlankNode represents a blank node value.
type BlankNode struct {
	Attribute string
}

// NewBlankNode creates a new instance of BlankNode.
func NewBlankNode(attribute string) BlankNode {
	bn := BlankNode{
		Attribute: attribute,
	}

	return bn
}

// GetValue returns the node's value.
func (bn BlankNode) GetValue() string {
	return bn.Attribute
}

// Equal returns true if this node is equal to the given node.
func (bn BlankNode) Equal(n Node) bool {
	if obn, ok := n.(*BlankNode); ok {
		return bn.Attribute == obn.Attribute
	}

	return false
}

// IsBlankNode returns true if the given node is a blank node
func IsBlankNode(node Node) bool {
	_, isBlankNode := node.(BlankNode)
	return isBlankNode
}

// IsIRI returns true if the given node is an IRI node
func IsIRI(node Node) bool {
	_, isIRI := node.(IRI)
	return isIRI
}

// IsLiteral returns true if the given node is a literal node
func IsLiteral(node Node) bool {
	_, isLiteral := node.(Literal)
	return isLiteral
}

var patternInteger = regexp.MustCompile(`^[\-+]?\d+$`)
var patternDouble = regexp.MustCompile(`^(\+|-)?(\d+(\.\d*)?|\.\d+)([Ee](\+|-)?\d+)?$`)

// RdfToObject converts an RDF triple object to a JSON-LD object.
func RdfToObject(n Node, useNativeTypes bool) (map[string]interface{}, error) {
	// If value is an an IRI or a blank node identifier, return a new
	// JSON object consisting
	// of a single member @id whose value is set to value.
	if IsIRI(n) || IsBlankNode(n) {
		return map[string]interface{}{
			"@id": n.GetValue(),
		}, nil
	}

	literal := n.(Literal)

	// convert literal object to JSON-LD
	rval := map[string]interface{}{
		"@value": literal.GetValue(),
	}

	// add language
	if literal.Language != "" {
		rval["@language"] = literal.Language
	} else {
		// add datatype
		datatype := literal.Datatype
		value := literal.Value
		if useNativeTypes {
			// use native datatypes for certain xsd types
			if datatype == XSDString {
				// don't add xsd:string
			} else if datatype == XSDBoolean {
				if value == "true" {
					rval["@value"] = true
				} else if value == "false" {
					rval["@value"] = false
				} else {
					// Else do not replace the value, and add the
					// boolean type in
					rval["@type"] = datatype
				}
			} else if (datatype == XSDInteger && patternInteger.MatchString(value)) /* http://www.w3.org/TR/xmlschema11-2/#integer */ ||
				(datatype == XSDDouble && patternDouble.MatchString(value)) /* http://www.w3.org/TR/xmlschema11-2/#nt-doubleRep */ {
				d, _ := strconv.ParseFloat(value, 64)
				if !math.IsNaN(d) && !math.IsInf(d, 0) {
					if datatype == XSDInteger {
						i := int64(d)
						if fmt.Sprintf("%d", i) == value {
							rval["@value"] = i
						}
					} else if datatype == XSDDouble {
						rval["@value"] = d
					} else {
						return nil, NewJsonLdError(ParseError, nil)
					}
				}
			} else {
				// do not add xsd:string type
				rval["@type"] = datatype
			}
		} else if datatype != XSDString {
			rval["@type"] = datatype
		}
	}

	return rval, nil
}

// objectToRDF converts a JSON-LD value object to an RDF literal or a JSON-LD string or
// node object to an RDF resource.
func objectToRDF(item interface{}, issuer *IdentifierIssuer, graphName string, triples []*Quad) (Node, []*Quad) {
	// convert value object to RDF
	if IsValue(item) {
		itemMap := item.(map[string]interface{})
		value := itemMap["@value"]
		datatype := itemMap["@type"]

		if datatype == "@json" {
			datatype = RDFJSONLiteral
		}

		// convert to XSD datatypes as appropriate
		booleanVal, isBool := value.(bool)
		floatVal, isFloat := value.(float64)

		if !isBool && !isFloat {
			// if document was created using a standard JSON decoder from json package
			// we need to be careful with float and integer representations.
			// If the client code sets UseNumber() property of json.Decoder
			// (see https://golang.org/pkg/encoding/json/#Decoder.UseNumber )
			// the logic above for discovering floats and integers will fail
			// because they would be represented as json.Number and not float64.
			// The code below takes care of it so it doesn't matter
			// how the document was decoded from JSON.
			if number, isNumber := value.(json.Number); isNumber {
				var floatErr error
				floatVal, floatErr = number.Float64()
				isFloat = floatErr == nil
			}
		}

		isInteger := isFloat && floatVal == float64(int64(floatVal))

		datatypeStr, _ := datatype.(string)
		if isBool || isFloat {
			// convert to XSD datatype
			if isBool {
				if datatype == nil {
					return NewLiteral(strconv.FormatBool(booleanVal), XSDBoolean, ""), triples
				} else {
					return NewLiteral(strconv.FormatBool(booleanVal), datatypeStr, ""), triples
				}
			} else if (isFloat && !isInteger) || XSDDouble == datatypeStr {
				canonicalDouble := GetCanonicalDouble(floatVal)
				if datatype == nil {
					return NewLiteral(canonicalDouble, XSDDouble, ""), triples
				} else {
					return NewLiteral(canonicalDouble, datatypeStr, ""), triples
				}
			} else {
				if datatype == nil {
					return NewLiteral(fmt.Sprintf("%d", int64(floatVal)), XSDInteger, ""), triples
				} else {
					return NewLiteral(fmt.Sprintf("%d", int64(floatVal)), datatype.(string), ""), triples
				}
			}
		} else if langVal, hasLang := itemMap["@language"]; hasLang {
			if datatype == nil {
				return NewLiteral(value.(string), RDFLangString, langVal.(string)), triples
			} else {
				return NewLiteral(value.(string), datatype.(string), langVal.(string)), triples
			}
		} else {
			if datatype == nil {
				return NewLiteral(value.(string), XSDString, ""), triples
			} else {
				if datatype != RDFJSONLiteral {
					return NewLiteral(value.(string), datatype.(string), ""), triples
				} else {
					var jsonLiteralValByte []byte
					switch v := value.(type) {
					case string:
						jsonLiteralValByte = []byte(v)
					case map[string]interface{}:
						byteVal, err := json.Marshal(v)
						if err != nil {
							return NewLiteral("JSON Marshal error "+err.Error(), datatype.(string), ""), triples
						}

						jsonLiteralValByte = byteVal
					}

					canonicalJSON, err := jsoncanonicalizer.Transform(jsonLiteralValByte)
					if err != nil {
						return NewLiteral("JSON Canonicalization error "+err.Error(), datatype.(string), ""), triples
					}

					return NewLiteral(string(canonicalJSON), datatype.(string), ""), triples
				}
			}
		}
	} else if IsList(item) {
		// if item is a list object, initialize list_results as an empty array,
		// and object to the result of the List Conversion algorithm, passing
		// the value associated with the @list key from item and list_results.
		return parseList(item.(map[string]interface{})["@list"].([]interface{}), issuer, graphName, triples)
	} else {
		// convert string/node object to RDF
		var id string
		if itemMap, isMap := item.(map[string]interface{}); isMap {
			id = itemMap["@id"].(string)
			if IsRelativeIri(id) {
				return nil, triples
			}
		} else {
			id = item.(string)
		}
		if strings.Index(id, "_:") == 0 {
			// NOTE: once again no need to rename existing blank nodes
			return NewBlankNode(id), triples
		} else {
			return NewIRI(id), triples
		}
	}
}

func parseList(list []interface{}, issuer *IdentifierIssuer, graphName string, triples []*Quad) (Node, []*Quad) {

	var res Node
	var last interface{}

	// is result is the head of the list?
	if len(list) > 0 {
		last = list[len(list)-1]
		res = NewBlankNode(issuer.GetId(""))
	} else {
		res = nilIRI
	}
	subj := res

	var obj Node
	for i := 0; i < len(list)-1; i++ {
		obj, triples = objectToRDF(list[i], issuer, graphName, triples)
		next := NewBlankNode(issuer.GetId(""))
		triples = append(triples,
			NewQuad(subj, first, obj, graphName),
			NewQuad(subj, rest, next, graphName),
		)
		subj = next
	}

	// tail of list
	if last != nil {
		obj, triples = objectToRDF(last, issuer, graphName, triples)
		triples = append(triples,
			NewQuad(subj, first, obj, graphName),
			NewQuad(subj, rest, nilIRI, graphName),
		)
	}

	return res, triples
}
