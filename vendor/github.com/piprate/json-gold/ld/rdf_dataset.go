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
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// Quad represents an RDF quad.
type Quad struct {
	Subject   Node
	Predicate Node
	Object    Node
	Graph     Node
}

// NewQuad creates a new instance of Quad.
func NewQuad(subject Node, predicate Node, object Node, graph string) *Quad {
	q := &Quad{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
	}

	if graph != "" && graph != "@default" {
		// TODO: i'm not yet sure if this should be added or if the
		// graph should only be represented by the keys in the dataset
		if strings.HasPrefix(graph, "_:") {
			q.Graph = NewBlankNode(graph)
		} else {
			q.Graph = NewIRI(graph)
		}
	}
	return q
}

// Equal returns true if this quad is equal to the given quad.
func (q *Quad) Equal(o *Quad) bool {
	if o == nil {
		return false
	}

	if (q.Graph != nil && !q.Graph.Equal(o.Graph)) || (q.Graph == nil && o.Graph != nil) {
		return false
	}

	return q.Subject.Equal(o.Subject) && q.Predicate.Equal(o.Predicate) && q.Object.Equal(o.Object)
}

func (q *Quad) Valid() bool {
	if q.Subject != nil {
		if InvalidNode(q.Subject) {
			return false
		}
	}

	if q.Predicate != nil {
		if InvalidNode(q.Predicate) {
			return false
		}
	}

	if q.Object != nil {
		if InvalidNode(q.Object) {
			return false
		}
	}

	if q.Graph != nil {
		if InvalidNode(q.Graph) {
			return false
		}
	}

	return true
}

// RDFDataset is an internal representation of an RDF dataset.
type RDFDataset struct {
	Graphs map[string][]*Quad

	context map[string]string
}

// RDFSerializer can serialize and de-serialize RDFDatasets.
type RDFSerializer interface {
	// Parse the input into the internal RDF Dataset format.
	// The format is a map with the following structure:
	// {
	// 	   GRAPH_1: [ TRIPLE_1, TRIPLE_2, ..., TRIPLE_N ],
	//     GRAPH_2: [ TRIPLE_1, TRIPLE_2, ..., TRIPLE_N ],
	//     ...
	//     GRAPH_N: [ TRIPLE_1, TRIPLE_2, ..., TRIPLE_N ]
	// }
	//
	// GRAPH: Must be the graph name/IRI. If no graph is present for a triple,
	// add it to the "@default" graph TRIPLE: Must be a map with the following
	// structure:
	// {
	//     "subject" : SUBJECT,
	//     "predicate" : PREDICATE,
	//     "object" : OBJECT
	// }
	//
	// Each of the values in the triple map must also be a map with the
	// following key-value pairs:
	//
	// "value": The value of the node.
	// "subject" can be an IRI or blank node id.
	// "predicate" should only ever be an IRI
	// "object" can be and IRI or blank node id, or a literal value (represented
	//     as a string)
	// "type": "IRI" if the value is an IRI or "blank node" if the
	// value is a blank node. "object" can also be "literal" in the case of
	// literals. The value of "object" can also contain the following optional
	// key-value pairs:
	//
	// "language" : the language value of a string literal
	// "datatype" : the datatype of the literal. (if not set will default to XSD:string,
	//     if set to null, null will be used).
	//
	Parse(input interface{}) (*RDFDataset, error)

	// Serialize an RDFDataset
	Serialize(dataset *RDFDataset) (interface{}, error)
}

// RDFSerializerTo can serialize RDFDatasets into io.Writer.
type RDFSerializerTo interface {
	SerializeTo(w io.Writer, dataset *RDFDataset) error
}

// NewRDFDataset creates a new instance of RDFDataset.
func NewRDFDataset() *RDFDataset {
	ds := &RDFDataset{
		context: make(map[string]string),
	}

	ds.Graphs = make(map[string][]*Quad)
	ds.Graphs["@default"] = make([]*Quad, 0)

	return ds
}

// SetNamespace
func (ds *RDFDataset) SetNamespace(ns string, prefix string) {
	ds.context[ns] = prefix
}

// GetNamespace
func (ds *RDFDataset) GetNamespace(ns string) string {
	return ds.context[ns]
}

// ClearNamespaces clears all the namespaces in this dataset
func (ds *RDFDataset) ClearNamespaces() {
	ds.context = make(map[string]string)
}

// GetNamespaces
func (ds *RDFDataset) GetNamespaces() map[string]string {
	return ds.context
}

// GetContext returns a valid context containing any namespaces set.
func (ds *RDFDataset) GetContext() map[string]interface{} {
	rval := make(map[string]interface{})
	for k, v := range ds.context {
		if k == "" {
			// replace "" with "@vocab"
			rval["@vocab"] = v
		} else {
			rval[k] = v
		}
	}
	return rval
}

// ParseContext parses a context object and sets any namespaces found within it.
func (ds *RDFDataset) ParseContext(contextLike interface{}, opts *JsonLdOptions) error {
	context := NewContext(nil, opts)

	// Context will do our recursive parsing and initial IRI resolution
	context, _ = context.Parse(contextLike)
	// And then leak to us the potential 'prefixes'
	prefixes := context.GetPrefixes(true)

	for key, val := range prefixes {
		if key == "@vocab" {
			ds.SetNamespace("", val)
		} else if !IsKeyword(key) {
			ds.SetNamespace(key, val)
			// TODO: should we make sure val is a valid URI prefix (i.e. it
			// ends with /# or ?)
			// or is it ok that full URIs for terms are used?
		}
	}
	return nil
}

var first = NewIRI(RDFFirst)
var rest = NewIRI(RDFRest)
var nilIRI = NewIRI(RDFNil)

// GraphToRDF creates an array of RDF triples for the given graph.
func (ds *RDFDataset) GraphToRDF(graphName string, graph map[string]interface{}, issuer *IdentifierIssuer,
	produceGeneralizedRdf bool) {
	// 4.2)
	triples := make([]*Quad, 0)
	// 4.3)
	for _, id := range GetKeys(graph) {
		if IsRelativeIri(id) {
			continue
		}

		node := graph[id].(map[string]interface{})
		for _, property := range GetOrderedKeys(node) {
			var values []interface{}
			// 4.3.2.1)
			if property == "@type" {
				values = node["@type"].([]interface{})
				property = RDFType
			} else if IsKeyword(property) {
				// 4.3.2.2)
				continue
			} else if strings.HasPrefix(property, "_:") && !produceGeneralizedRdf {
				// 4.3.2.3)
				continue
			} else if IsRelativeIri(property) {
				// 4.3.2.4)
				continue
			} else {
				values = node[property].([]interface{})
			}

			var subject Node
			if strings.Index(id, "_:") == 0 {
				// NOTE: don't rename, just set it as a blank node
				subject = NewBlankNode(id)
			} else {
				subject = NewIRI(id)
			}

			// RDF predicates
			var predicate Node
			if strings.HasPrefix(property, "_:") {
				predicate = NewBlankNode(property)
			} else {
				predicate = NewIRI(property)
			}

			for _, item := range values {
				var object Node
				object, triples = objectToRDF(item, issuer, graphName, triples)
				if object != nil {
					triples = append(triples, NewQuad(subject, predicate, object, graphName))
				}
			}
		}
	}

	// drop invalid statements (other than IRIs)
	sanitisedTriples := make([]*Quad, 0, len(triples))
	for _, t := range triples {
		if t.Valid() {
			sanitisedTriples = append(sanitisedTriples, t)
		}
	}
	ds.Graphs[graphName] = sanitisedTriples
}

// GetQuads returns a list of quads for the given graph
func (ds *RDFDataset) GetQuads(graphName string) []*Quad {
	return ds.Graphs[graphName]
}

var canonicalDoubleRegEx = regexp.MustCompile(`(\d)0*E\+?(-)?0*(\d)`)

// GetCanonicalDouble returns a canonical string representation of a float64 number.
func GetCanonicalDouble(v float64) string {
	return canonicalDoubleRegEx.ReplaceAllString(fmt.Sprintf("%1.15E", v), "${1}E${2}${3}")
}

var (
	validLanguageRegex = regexp.MustCompile("^[a-zA-Z]+(-[a-zA-Z0-9]+)*$")
)

func InvalidNode(node Node) bool {

	switch v := node.(type) {
	case IRI:
		if !validIRI(v.Value) {
			return true
		}
	case Literal:
		if v.Language != "" && !validLanguageRegex.MatchString(v.Language) {
			return true
		}
		if v.Datatype != "" && !validIRI(v.Datatype) {
			return true
		}
	}

	return false
}

func validIRI(val string) bool {
	if (strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://")) && !IsURL(val) {
		return false
	}

	return true
}

/*
===========
The URL validation logic below was borrowed from github.com/asaskevich/govalidator package.
The original code is distributed under MIT license. Copyright (c) 2014 Alex Saskevich
===========
*/
var (
	IP           = `(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`
	URLSchema    = `((ftp|tcp|udp|wss?|https?):\/\/)`
	URLUsername  = `(\S+(:\S*)?@)`
	URLPath      = `((\/|\?|#)[^\s]*)`
	URLPort      = `(:(\d{1,5}))`
	URLIP        = `([1-9]\d?|1\d\d|2[01]\d|22[0-3])(\.(1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.([0-9]\d?|1\d\d|2[0-4]\d|25[0-4]))`
	URLSubdomain = `((www\.)|([a-zA-Z0-9]+([-_\.]?[a-zA-Z0-9])*[a-zA-Z0-9]\.[a-zA-Z0-9]+))`
	URL          = `^` + URLSchema + `?` + URLUsername + `?` + `((` + URLIP + `|(\[` + IP + `\])|(([a-zA-Z0-9]([a-zA-Z0-9-_]+)?[a-zA-Z0-9]([-\.][a-zA-Z0-9]+)*)|(` + URLSubdomain + `?))?(([a-zA-Z\x{00a1}-\x{ffff}0-9]+-?-?)*[a-zA-Z\x{00a1}-\x{ffff}0-9]+)(?:\.([a-zA-Z\x{00a1}-\x{ffff}]{1,}))?))\.?` + URLPort + `?` + URLPath + `?$`
	rxURL        *regexp.Regexp
	onlyOnce     sync.Once
)

const maxURLRuneCount = 2083
const minURLRuneCount = 3

// IsURL check if the string is an URL.
func IsURL(str string) bool {
	onlyOnce.Do(func() {
		rxURL = regexp.MustCompile(URL)
	})
	if str == "" || utf8.RuneCountInString(str) >= maxURLRuneCount || len(str) <= minURLRuneCount || strings.HasPrefix(str, ".") {
		return false
	}
	strTemp := str
	if strings.Contains(str, ":") && !strings.Contains(str, "://") {
		// support no indicated urlscheme but with colon for port number
		// http:// is appended so url.Parse will succeed, strTemp used so it does not impact rxURL.MatchString
		strTemp = "http://" + str
	}
	u, err := url.Parse(strTemp)
	if err != nil {
		return false
	}
	if strings.HasPrefix(u.Host, ".") {
		return false
	}
	if u.Host == "" && (u.Path != "" && !strings.Contains(u.Path, ".")) {
		return false
	}
	return rxURL.MatchString(str)
}
