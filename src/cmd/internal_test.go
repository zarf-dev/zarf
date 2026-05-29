// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFenceIndentedCodeBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "tab-indented block becomes fenced",
			in:   "Run this:\n\n\tsource <(zarf completion zsh)\n\nDone.\n",
			want: "Run this:\n\n```\nsource <(zarf completion zsh)\n```\n\nDone.\n",
		},
		{
			name: "four-space-indented block becomes fenced",
			in:   "Example:\n\n    echo hi\n\nEnd.\n",
			want: "Example:\n\n```\necho hi\n```\n\nEnd.\n",
		},
		{
			name: "multi-line indented block preserves internal blanks",
			in:   "Steps:\n\n\tone\n\n\ttwo\n\nFin.\n",
			want: "Steps:\n\n```\none\n\ntwo\n```\n\nFin.\n",
		},
		{
			name: "existing fenced block is untouched",
			in:   "```\n  already fenced <x>\n```\n",
			want: "```\n  already fenced <x>\n```\n",
		},
		{
			name: "indentation inside an existing fence is not refenced",
			in:   "```sh\n\tindented inside fence\n```\n",
			want: "```sh\n\tindented inside fence\n```\n",
		},
		{
			name: "non-indented prose is unchanged",
			in:   "Just a paragraph.\nAnother line.\n",
			want: "Just a paragraph.\nAnother line.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, fenceIndentedCodeBlocks(tt.in))
		})
	}
}
