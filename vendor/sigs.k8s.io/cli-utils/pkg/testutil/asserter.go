// Copyright 2020 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
)

// Asserter provides a set of assertion methods that use a shared set of
// comparison options.
type Asserter struct {
	Options cmp.Options
}

// Returns a new Asserter with the specified options.
func NewAsserter(opts ...cmp.Option) *Asserter {
	return &Asserter{
		Options: opts,
	}
}

// DefaultAsserter is a global Asserter with default comparison options:
// - EquateErrors (compare with "Is(T) bool" method)
var DefaultAsserter = NewAsserter(cmpopts.EquateErrors())

// EqualMatcher returns a new EqualMatcher with the Asserter's options and the
// specified expected value.
func (a *Asserter) EqualMatcher(expected interface{}) *EqualMatcher {
	return &EqualMatcher{
		Expected: expected,
		Options:  a.Options,
	}
}

// Equal fails the test if the actual value does not deeply equal the
// expected value. Prints a diff on failure.
func (a *Asserter) Equal(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper() // print the caller's file:line, instead of this func, on failure
	matcher := a.EqualMatcher(expected)
	match, err := matcher.Match(actual)
	if err != nil {
		t.Errorf("errored testing equality: %v", err)
		return
	}
	if !match {
		assert.Fail(t, matcher.FailureMessage(actual), msgAndArgs...)
	}
}

// AssertEqual fails the test if the actual value does not deeply equal the
// expected value. Prints a diff on failure.
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper() // print the caller's file:line, instead of this func, on failure
	DefaultAsserter.Equal(t, expected, actual, msgAndArgs...)
}

// NotEqual fails the test if the actual value deeply equals the
// expected value. Prints a diff on failure.
func (a *Asserter) NotEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper() // print the caller's file:line, instead of this func, on failure
	matcher := a.EqualMatcher(expected)
	match, err := matcher.Match(actual)
	if err != nil {
		t.Errorf("errored testing equality: %v", err)
		return
	}
	if match {
		assert.Fail(t, matcher.NegatedFailureMessage(actual), msgAndArgs...)
	}
}

// AssertNotEqual fails the test if the actual value deeply equals the
// expected value. Prints a diff on failure.
func AssertNotEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper() // print the caller's file:line, instead of this func, on failure
	DefaultAsserter.NotEqual(t, expected, actual, msgAndArgs...)
}
