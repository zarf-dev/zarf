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

const (
	RDFSyntaxNS string = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFSchemaNS string = "http://www.w3.org/2000/01/rdf-schema#"
	XSDNS       string = "http://www.w3.org/2001/XMLSchema#"

	XSDAnyType string = XSDNS + "anyType"
	XSDBoolean string = XSDNS + "boolean"
	XSDDouble  string = XSDNS + "double"
	XSDInteger string = XSDNS + "integer"
	XSDFloat   string = XSDNS + "float"
	XSDDecimal string = XSDNS + "decimal"
	XSDAnyURI  string = XSDNS + "anyURI"
	XSDString  string = XSDNS + "string"

	RDFType         string = RDFSyntaxNS + "type"
	RDFFirst        string = RDFSyntaxNS + "first"
	RDFRest         string = RDFSyntaxNS + "rest"
	RDFNil          string = RDFSyntaxNS + "nil"
	RDFPlainLiteral string = RDFSyntaxNS + "PlainLiteral"
	RDFXMLLiteral   string = RDFSyntaxNS + "XMLLiteral"
	RDFJSONLiteral  string = RDFSyntaxNS + "JSON"
	RDFObject       string = RDFSyntaxNS + "object"
	RDFLangString   string = RDFSyntaxNS + "langString"
	RDFList         string = RDFSyntaxNS + "List"
)
