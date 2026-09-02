// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2021 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2021 Intevation GmbH <https://intevation.de>

package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Intevation/gval"
	"github.com/Intevation/jsonpath"
)

// ReMarshalJSON transforms data from src to dst via JSON marshalling.
func ReMarshalJSON(dst, src any) error {
	intermediate, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(intermediate, dst)
}

// PathEval is a helper to evaluate JSON paths on documents.
type PathEval struct {
	builder gval.Language
	exprs   map[string]gval.Evaluable
}

// NewPathEval creates a new PathEval.
func NewPathEval() *PathEval {
	return &PathEval{
		builder: gval.Full(jsonpath.Language()),
		exprs:   map[string]gval.Evaluable{},
	}
}

// Compile compiles an expression and stores it in the
// internal cache on success.
func (pe *PathEval) Compile(expr string) (gval.Evaluable, error) {
	if eval := pe.exprs[expr]; eval != nil {
		return eval, nil
	}
	eval, err := pe.builder.NewEvaluable(expr)
	if err != nil {
		return nil, err
	}
	pe.exprs[expr] = eval
	return eval, nil
}

// Eval evalutes expression expr on document doc.
// Returns the result of the expression.
func (pe *PathEval) Eval(expr string, doc any) (any, error) {
	if doc == nil {
		return nil, errors.New("no document to extract data from")
	}
	eval := pe.exprs[expr]
	if eval == nil {
		var err error
		if eval, err = pe.builder.NewEvaluable(expr); err != nil {
			return nil, err
		}
		pe.exprs[expr] = eval
	}
	return eval(context.Background(), doc)
}

// PathEvalMatcher is a pair of an expression and an action
// when doing extractions via PathEval.Match.
type PathEvalMatcher struct {
	// Expr is the expression to evaluate
	Expr string
	// Action is executed with the result of the match.
	Action func(any) error
	// Optional expresses if the expression is optional.
	Optional bool
}

// ReMarshalMatcher is an action to re-marshal the result to another type.
func ReMarshalMatcher(dst any) func(any) error {
	return func(src any) error {
		return ReMarshalJSON(dst, src)
	}
}

// BoolMatcher stores the matched result in a bool.
func BoolMatcher(dst *bool) func(any) error {
	return func(x any) error {
		b, ok := x.(bool)
		if !ok {
			return errors.New("not a bool")
		}
		*dst = b
		return nil
	}
}

// StringMatcher stores the matched result in a string.
func StringMatcher(dst *string) func(any) error {
	return func(x any) error {
		s, ok := x.(string)
		if !ok {
			return errors.New("not a string")
		}
		*dst = s
		return nil
	}
}

// StringTreeMatcher returns a matcher which adds strings
// to a slice and recursively strings from arrays of strings.
func StringTreeMatcher(strings *[]string) func(any) error {
	// Only add unique strings.
	unique := func(s string) {
		for _, t := range *strings {
			if s == t {
				return
			}
		}
		*strings = append(*strings, s)
	}
	var recurse func(any) error
	recurse = func(x any) error {
		switch y := x.(type) {
		case string:
			unique(y)
		case []any:
			for _, z := range y {
				if err := recurse(z); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported type: %T", x)
		}
		return nil
	}
	return recurse
}

// TimeMatcher stores a time with a given format.
func TimeMatcher(dst *time.Time, format string) func(any) error {
	return func(x any) error {
		s, ok := x.(string)
		if !ok {
			return errors.New("not a string")
		}
		t, err := time.Parse(format, s)
		if err != nil {
			return err
		}
		*dst = t
		return nil
	}
}

// Extract extracts a value from a given document with a given expression/action.
func (pe *PathEval) Extract(
	expr string,
	action func(any) error,
	optional bool,
	doc any,
) error {
	optErr := func(err error) error {
		if err == nil || optional {
			return nil
		}
		return fmt.Errorf("extract failed '%s': %v", expr, err)
	}
	x, err := pe.Eval(expr, doc)
	if err != nil {
		return optErr(err)
	}
	return optErr(action(x))
}

// Match matches a list of PathEvalMatcher pairs against a document.
func (pe *PathEval) Match(matcher []PathEvalMatcher, doc any) error {
	for _, m := range matcher {
		if err := pe.Extract(m.Expr, m.Action, m.Optional, doc); err != nil {
			return err
		}
	}
	return nil
}

// Strings searches the given document for the given set of expressions
// and returns the corresponding strings. The optional flag indicates
// if the expression evaluation have to succseed or not.
func (pe *PathEval) Strings(
	exprs []string,
	optional bool,
	doc any,
) ([]string, error) {
	results := make([]string, 0, len(exprs))
	var result string
	matcher := StringMatcher(&result)
	for _, expr := range exprs {
		if err := pe.Extract(expr, matcher, optional, doc); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

// AsStrings transforms an []interface{string, string,... }
// to a []string.
func AsStrings(x any) ([]string, bool) {
	strs, ok := x.([]any)
	if !ok {
		return nil, false
	}
	res := make([]string, 0, len(strs))
	for _, y := range strs {
		if s, ok := y.(string); ok {
			res = append(res, s)
		}
	}
	return res, true
}
