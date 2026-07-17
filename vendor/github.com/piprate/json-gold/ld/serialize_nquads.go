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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// NQuadRDFSerializer parses and serializes N-Quads.
type NQuadRDFSerializer struct {
}

// Parse N-Quads from string into an RDFDataset.
func (s *NQuadRDFSerializer) Parse(input interface{}) (*RDFDataset, error) {
	return ParseNQuadsFrom(input)
}

// SerializeTo writes RDFDataset as N-Quad into a writer.
func (s *NQuadRDFSerializer) SerializeTo(w io.Writer, dataset *RDFDataset) error {
	for graphName, triples := range dataset.Graphs {
		if graphName == "@default" {
			graphName = ""
		}
		for _, triple := range triples {
			quad := toNQuad(triple, graphName)
			if _, err := fmt.Fprint(w, quad); err != nil {
				return NewJsonLdError(IOError, err)
			}
		}
	}
	return nil
}

// Serialize an RDFDataset into N-Quad string.
func (s *NQuadRDFSerializer) Serialize(dataset *RDFDataset) (interface{}, error) {
	buf := bytes.NewBuffer(nil)
	if err := s.SerializeTo(buf, dataset); err != nil {
		return nil, err
	}
	return buf.String(), nil
}

func toNQuad(triple *Quad, graphName string) string {

	s := triple.Subject
	p := triple.Predicate
	o := triple.Object

	quad := ""

	// subject is an IRI or bnode
	if IsIRI(s) {
		quad += "<" + escape(s.GetValue()) + ">"
	} else {
		quad += s.GetValue()
	}

	if IsIRI(p) {
		quad += " <" + escape(p.GetValue()) + "> "
	} else {
		quad += " " + escape(p.GetValue()) + " "
	}

	// object is IRI, bnode or literal
	if IsIRI(o) {
		quad += "<" + escape(o.GetValue()) + ">"
	} else if IsBlankNode(o) {
		quad += o.GetValue()
	} else {
		literal := o.(Literal)
		escaped := escape(literal.GetValue())
		quad += "\"" + escaped + "\""
		if literal.Datatype == RDFLangString {
			quad += "@" + literal.Language
		} else if literal.Datatype != XSDString {
			quad += "^^<" + escape(literal.Datatype) + ">"
		}
	}

	// graph
	if graphName != "" {
		if strings.Index(graphName, "_:") != 0 {
			quad += " <" + escape(graphName) + ">"
		} else {
			quad += " " + graphName
		}
	}

	quad += " .\n"

	return quad
}

func unescape(str string) string {
	str = strings.ReplaceAll(str, "\\\\", "\\")
	str = strings.ReplaceAll(str, "\\\"", "\"")
	str = strings.ReplaceAll(str, "\\n", "\n")
	str = strings.ReplaceAll(str, "\\r", "\r")
	str = strings.ReplaceAll(str, "\\t", "\t")
	return str
}

func escape(str string) string {
	str = strings.ReplaceAll(str, "\\", "\\\\")
	str = strings.ReplaceAll(str, "\"", "\\\"")
	str = strings.ReplaceAll(str, "\n", "\\n")
	str = strings.ReplaceAll(str, "\r", "\\r")
	str = strings.ReplaceAll(str, "\t", "\\t")
	return str
}

const (
	wso = "[ \\t]*"
	iri = "(?:<([^:]+:[^>]*)>)"

	// https://www.w3.org/TR/turtle/#grammar-production-BLANK_NODE_LABEL

	pnCharsBase = "A-Z" + "a-z" +
		"\u00C0-\u00D6" +
		"\u00D8-\u00F6" +
		"\u00F8-\u02FF" +
		"\u0370-\u037D" +
		"\u037F-\u1FFF" +
		"\u200C-\u200D" +
		"\u2070-\u218F" +
		"\u2C00-\u2FEF" +
		"\u3001-\uD7FF" +
		"\uF900-\uFDCF" +
		"\uFDF0-\uFFFD"
	// TODO:
	//"\u10000-\uEFFFF"

	pnCharsU = pnCharsBase + "_"

	pnChars = pnCharsU +
		"0-9" +
		"-" +
		"\u00B7" +
		"\u0300-\u036F" +
		"\u203F-\u2040"

	blankNodeLabel = "(_:" +
		"(?:[" + pnCharsU + "0-9])" +
		"(?:(?:[" + pnChars + ".])*(?:[" + pnChars + "]))?" +
		")"

	//   '(_:' +
	//     '(?:[' + PN_CHARS_U + '0-9])' +
	//     '(?:(?:[' + PN_CHARS + '.])*(?:[' + PN_CHARS + ']))?' +
	//   ')';

	bnode = blankNodeLabel

	plain    = "\"([^\"\\\\]*(?:\\\\.[^\"\\\\]*)*)\""
	datatype = "(?:\\^\\^" + iri + ")"
	language = "(?:@([a-z]+(?:-[a-zA-Z0-9]+)*))"
	literal  = "(?:" + plain + "(?:" + datatype + "|" + language + ")?)"
	ws       = "[ \\t]+"

	subject  = "(?:" + iri + "|" + bnode + ")" + ws
	property = iri + ws
	object   = "(?:" + iri + "|" + bnode + "|" + literal + ")" + wso
	graph    = "(?:\\.|(?:(?:" + iri + "|" + bnode + ")" + wso + "\\.))"
)

var regexEmpty = regexp.MustCompile("^" + wso + "$")

// full quad regex

var regexQuad = regexp.MustCompile("^" + wso + subject + property + object + graph + wso + "$") //nolint:gocritic

type lineScanner interface {
	Bytes() []byte
	Scan() bool
	Err() error
}

type bytesLineScanner struct {
	err   error
	b     []byte
	token []byte
	i     int
}

func (ls *bytesLineScanner) Err() error { return ls.err }
func (ls *bytesLineScanner) Scan() bool {
	b, i := ls.b, ls.i
	if ls.err != nil || i >= len(b) {
		return false
	}
	di, token, err := bufio.ScanLines(b[i:], true)
	if err != nil {
		ls.err = err
		return false
	}
	ls.token = token
	ls.i += di
	return true
}
func (ls *bytesLineScanner) Bytes() []byte {
	return ls.token
}

func newScannerFor(o interface{}) (lineScanner, error) {
	switch inp := o.(type) {
	case []byte:
		return &bytesLineScanner{b: inp}, nil
	case string:
		return &bytesLineScanner{b: []byte(inp)}, nil
	case io.Reader:
		return bufio.NewScanner(inp), nil
	default:
		return nil, NewJsonLdError(InvalidInput, "expected []byte, string or io.Reader")
	}
}

// ParseNQuadsFrom parses RDF in the form of N-Quads from io.Reader, []byte or string.
func ParseNQuadsFrom(o interface{}) (*RDFDataset, error) {

	// build RDF dataset
	dataset := NewRDFDataset()

	// maintain a set of triples for each graph to check for duplicates
	triplesByGraph := make(map[string]map[Quad]struct{})

	scanner, err := newScannerFor(o)
	if err != nil {
		return nil, err
	}

	// scan N-Quad input lines
	lineNumber := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		lineNumber++

		// skip empty lines
		if regexEmpty.Match(line) {
			continue
		}

		// parse quad
		if !regexQuad.Match(line) {
			return nil, NewJsonLdError(SyntaxError, fmt.Errorf("error while parsing N-Quads; invalid quad. line: %d", lineNumber))
		}
		match := regexQuad.FindStringSubmatch(string(line))

		// get subject
		var subject Node
		if match[1] != "" {
			subject = NewIRI(unescape(match[1]))
		} else {
			subject = NewBlankNode(unescape(match[2]))
		}

		// get predicate
		predicate := NewIRI(unescape(match[3]))

		// get object
		var object Node
		if match[4] != "" {
			object = NewIRI(unescape(match[4]))
		} else if match[5] != "" {
			object = NewBlankNode(unescape(match[5]))
		} else {
			language := unescape(match[8])
			var datatype string
			if match[7] != "" {
				datatype = unescape(match[7])
			} else if match[8] != "" {
				datatype = RDFLangString
			} else {
				datatype = XSDString
			}
			unescaped := unescape(match[6])
			object = NewLiteral(unescaped, datatype, language)
		}

		// get graph name ('@default' is used for the default graph)
		name := "@default"
		if match[9] != "" {
			name = unescape(match[9])
		} else if match[10] != "" {
			name = unescape(match[10])
		}

		triple := NewQuad(subject, predicate, object, name)

		// initialise graph in dataset
		triples, present := dataset.Graphs[name]
		if triplesByGraph[name] == nil {
			triplesByGraph[name] = make(map[Quad]struct{})
		}

		if !present {
			dataset.Graphs[name] = []*Quad{triple}
		} else {
			// add triple if unique to its graph
			if _, hasTriple := triplesByGraph[name][*triple]; !hasTriple {
				dataset.Graphs[name] = append(triples, triple)
			}
		}
		triplesByGraph[name][*triple] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, NewJsonLdError(IOError, err)
	}

	return dataset, nil
}

// ParseNQuads parses RDF in the form of N-Quads.
func ParseNQuads(input string) (*RDFDataset, error) {
	return ParseNQuadsFrom(input)
}
