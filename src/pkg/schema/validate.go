// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package schema

import (
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// ValidationError contains schema violations with their native field paths and descriptions.
type ValidationError struct {
	Errors []gojsonschema.ResultError
}

func (e *ValidationError) Error() string {
	messages := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		messages = append(messages, err.String())
	}
	return "schema validation failed:\n" + strings.Join(messages, "\n")
}

// Validate checks document against schemaBytes and returns a ValidationError for schema violations.
// Use an untyped document to retain unknown fields for validation.
func Validate(schemaBytes []byte, document any) error {
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	documentLoader := gojsonschema.NewGoLoader(document)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		return &ValidationError{Errors: result.Errors()}
	}
	return nil
}
