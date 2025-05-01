// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive contains the SDK for Zarf archival and compression.
package archive

import (
	"testing"

	"github.com/open-policy-agent/opa/util/test"
)

// TODO(mkcp): Unit test Compress
func TestCompress(t *testing.T) {
	test.Skip(t)
	tt := []struct {
		name string
		opts CompressOpts
	}{
		{
			name: "CompressOpts can be empty",
			opts: CompressOpts{},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Skip()
		})
	}
}

// TODO(mkcp): Unit test Decompress. Some overlap with e2e/05_tarball_test.go
func TestDecompress(t *testing.T) {
	tt := []struct {
		name string
		opts DecompressOpts
	}{
		{
			name: "TODO",
			opts: DecompressOpts{
				UnarchiveAll: true,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Skip()
		})
	}
}
