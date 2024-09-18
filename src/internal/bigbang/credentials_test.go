// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCredentials(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "Test Big Bang v1 Zarf Credentials",
			version:  "1.55.0",
			expected: bbV1ZarfCredentialsValues,
		},
		{
			name:     "Test Big Bang v2 Zarf Credentials",
			version:  "2.22.0",
			expected: bbV2ZarfCredentialsValues,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			creds, err := manifestZarfCredentials(tt.version)
			require.NoError(t, err)
			require.Equal(t, tt.expected, creds)
		})
	}
}
