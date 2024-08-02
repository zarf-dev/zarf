// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

func TestParseRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		refPlain    string
		expectedRef plumbing.ReferenceName
	}{
		{
			name:        "basic",
			refPlain:    "v1.0.0",
			expectedRef: plumbing.ReferenceName("refs/tags/v1.0.0"),
		},
		{
			name:        "basic",
			refPlain:    "refs/heads/branchname",
			expectedRef: plumbing.ReferenceName("refs/heads/branchname"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ref := ParseRef(tt.refPlain)
			require.Equal(t, tt.expectedRef, ref)
		})
	}
}
