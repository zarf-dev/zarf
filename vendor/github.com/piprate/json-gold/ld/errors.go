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
)

// ErrorCode is a JSON-LD error code as per spec.
type ErrorCode string

// JsonLdError is a JSON-LD error as defined in the spec.
// See the allowed values and error messages below.
type JsonLdError struct { //nolint:stylecheck
	Code    ErrorCode
	Details interface{}
}

const (
	LoadingDocumentFailed       ErrorCode = "loading document failed"
	InvalidIndexValue           ErrorCode = "invalid @index value"
	ConflictingIndexes          ErrorCode = "conflicting indexes"
	InvalidIDValue              ErrorCode = "invalid @id value"
	InvalidLocalContext         ErrorCode = "invalid local context"
	MultipleContextLinkHeaders  ErrorCode = "multiple context link headers"
	LoadingRemoteContextFailed  ErrorCode = "loading remote context failed"
	InvalidRemoteContext        ErrorCode = "invalid remote context"
	RecursiveContextInclusion   ErrorCode = "recursive context inclusion"
	InvalidBaseIRI              ErrorCode = "invalid base IRI"
	InvalidVocabMapping         ErrorCode = "invalid vocab mapping"
	InvalidDefaultLanguage      ErrorCode = "invalid default language"
	KeywordRedefinition         ErrorCode = "keyword redefinition"
	InvalidTermDefinition       ErrorCode = "invalid term definition"
	InvalidReverseProperty      ErrorCode = "invalid reverse property"
	InvalidIRIMapping           ErrorCode = "invalid IRI mapping"
	CyclicIRIMapping            ErrorCode = "cyclic IRI mapping"
	InvalidKeywordAlias         ErrorCode = "invalid keyword alias"
	InvalidTypeMapping          ErrorCode = "invalid type mapping"
	InvalidLanguageMapping      ErrorCode = "invalid language mapping"
	CollidingKeywords           ErrorCode = "colliding keywords"
	InvalidContainerMapping     ErrorCode = "invalid container mapping"
	InvalidTypeValue            ErrorCode = "invalid type value"
	InvalidValueObject          ErrorCode = "invalid value object"
	InvalidValueObjectValue     ErrorCode = "invalid value object value"
	InvalidLanguageTaggedString ErrorCode = "invalid language-tagged string"
	InvalidLanguageTaggedValue  ErrorCode = "invalid language-tagged value"
	InvalidTypedValue           ErrorCode = "invalid typed value"
	InvalidSetOrListObject      ErrorCode = "invalid set or list object"
	InvalidLanguageMapValue     ErrorCode = "invalid language map value"
	InvalidReversePropertyMap   ErrorCode = "invalid reverse property map"
	InvalidReverseValue         ErrorCode = "invalid @reverse value"
	InvalidReversePropertyValue ErrorCode = "invalid reverse property value"
	InvalidVersionValue         ErrorCode = "invalid @version value"
	ProcessingModeConflict      ErrorCode = "processing mode conflict"
	InvalidFrame                ErrorCode = "invalid frame"
	InvalidEmbedValue           ErrorCode = "invalid @embed value"
	InvalidPrefixValue          ErrorCode = "invalid @prefix value"
	InvalidNestValue            ErrorCode = "invalid @nest value"
	InvalidContextNullification ErrorCode = "invalid context nullification"
	ProtectedTermRedefinition   ErrorCode = "protected term redefinition"
	InvalidContextEntry         ErrorCode = "invalid context entry"
	InvalidPropagateValue       ErrorCode = "invalid @propagate value"
	InvalidBaseDirection        ErrorCode = "invalid base direction"
	InvalidIncludedValue        ErrorCode = "invalid @included value"
	InvalidImportValue          ErrorCode = "invalid @import value"
	IRIConfusedWithPrefix       ErrorCode = "IRI confused with prefix"

	// non spec related errors
	SyntaxError     ErrorCode = "syntax error"
	NotImplemented  ErrorCode = "not implemented"
	UnknownFormat   ErrorCode = "unknown format"
	InvalidInput    ErrorCode = "invalid input"
	ParseError      ErrorCode = "parse error"
	IOError         ErrorCode = "io error"
	InvalidProperty ErrorCode = "invalid property"
	UnknownError    ErrorCode = "unknown error"
)

func (e JsonLdError) Error() string {
	if e.Details != nil {
		return fmt.Sprintf("%v: %v", e.Code, e.Details)
	}
	return fmt.Sprintf("%v", e.Code)
}

// Unwrap returns JsonLdError.Details if it is an error, otherwise nil.
func (e JsonLdError) Unwrap() error {
	cause, _ := e.Details.(error)
	return cause
}

// NewJsonLdError creates a new instance of JsonLdError.
func NewJsonLdError(code ErrorCode, details interface{}) *JsonLdError { //nolint:stylecheck
	return &JsonLdError{Code: code, Details: details}
}
