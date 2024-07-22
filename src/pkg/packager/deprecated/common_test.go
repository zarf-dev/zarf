// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"bytes"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

func TestPrintBreakingChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		deployedVersion string
		cliVersion      string
		breakingChanges []BreakingChange
	}{
		{
			name:            "No breaking changes",
			deployedVersion: "0.26.0",
			cliVersion:      "0.26.0",
			breakingChanges: []BreakingChange{},
		},
		{
			name:            "agent breaking change",
			deployedVersion: "0.25.0",
			cliVersion:      "0.26.0",
			breakingChanges: []BreakingChange{
				{
					version:    semver.MustParse("0.26.0"),
					title:      "Zarf container images are now mutated based on tag instead of repository name.",
					mitigation: "Reinitialize the cluster using v0.26.0 or later and redeploy existing packages to update the image references (you can view existing packages with 'zarf package list' and view cluster images with 'zarf tools registry catalog').",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			message.InitializePTerm(&output)
			err := PrintBreakingChanges(&output, tt.deployedVersion, tt.cliVersion)
			require.NoError(t, err)
			for _, bc := range tt.breakingChanges {
				require.Contains(t, output.String(), bc.String())
			}
			t.Log(output.String())
		})
	}
}
