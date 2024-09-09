// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByteFormat(t *testing.T) {
	t.Parallel()
	tt := []struct {
		name      string
		in        float64
		precision int
		expect    string
	}{
		{
			name:   "accepts empty",
			expect: "0.0 Byte",
		},
		{
			name:      "accepts empty bytes with precision",
			precision: 1,
			expect:    "0.0 Byte",
		},
		{
			name:      "accepts empty bytes with meaningful precision",
			precision: 3,
			expect:    "0.000 Byte",
		},
		{
			name:   "formats negative byte with empty precision",
			in:     -1,
			expect: "-1.0 Byte",
		},
		{
			name:   "formats negative bytes with empty precision",
			in:     -2,
			expect: "-2.0 Bytes",
		},
		{
			name:   "formats kilobyte",
			in:     1000,
			expect: "1.0 KB",
		},
		{
			name:   "formats kilobytes",
			in:     1100,
			expect: "1.1 KBs",
		},
		{
			name:   "formats megabytes",
			in:     10000000,
			expect: "10.0 MBs",
		},
		{
			name:   "formats gigabytes",
			in:     100000000000,
			expect: "100.0 GBs",
		},
		{
			name:      "formats arbitrary in",
			in:        4238970784923,
			precision: 99,
			expect:    "4238.970784922999882837757468223571777343750000000000000000000000000000000000000000000000000000000000000 GBs",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := ByteFormat(tc.in, tc.precision)
			require.Equal(t, tc.expect, actual)
		})
	}
}
