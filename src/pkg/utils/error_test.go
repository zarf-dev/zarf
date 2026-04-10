// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"errors"
	"testing"
)

type errorTypeA struct{ msg string }

func (e *errorTypeA) Error() string { return e.msg }

type errorTypeB struct{ msg string }

func (e *errorTypeB) Error() string { return e.msg }

func TestFilterErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          error
		run            func(error) (error, error)
		wantErr        bool
		wantResultNil  bool
		wantUnwrapLen  int
		expectedResult error
	}{
		{
			name:           "filters out matching error type",
			input:          errors.Join(&errorTypeA{msg: "a"}, &errorTypeB{msg: "b"}),
			run:            FilterErr[*errorTypeA],
			expectedResult: errors.Join(&errorTypeB{msg: "b"}),
		},
		{
			name:           "returns nil when all errors are filtered",
			input:          errors.Join(&errorTypeA{msg: "a"}),
			run:            FilterErr[*errorTypeA],
			expectedResult: nil,
		},
		{
			name:          "returns all errors when none match the type",
			input:         errors.Join(&errorTypeA{msg: "a1"}, &errorTypeA{msg: "a2"}),
			run:           FilterErr[*errorTypeB],
			wantUnwrapLen: 2,
		},
		{
			name:    "returns error when input does not implement Unwrap",
			input:   &errorTypeA{msg: "plain"},
			run:     FilterErr[*errorTypeB],
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := tc.run(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Error("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantResultNil && result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if tc.expectedResult != nil && result.Error() != tc.expectedResult.Error() {
				t.Errorf("expected error string %s, but got error string %s", tc.expectedResult.Error(), result.Error())
			}
			if tc.wantUnwrapLen > 0 {
				unwrapped, ok := result.(interface{ Unwrap() []error })
				if !ok {
					t.Fatal("expected result to be a joined error")
				}
				if got := len(unwrapped.Unwrap()); got != tc.wantUnwrapLen {
					t.Errorf("expected %d errors, got %d", tc.wantUnwrapLen, got)
				}
			}
		})
	}
}
