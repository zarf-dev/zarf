// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package operations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestShouldMutate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		resourceLabels map[string]string
		nsLabels       map[string]string
		mode           state.MutationMode
		want           bool
	}{
		// Opt-out: resource label takes priority
		{name: "opt-out/no labels", mode: state.MutationModeOptOut, want: true},
		{name: "opt-out/resource mutate", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationModeOptOut, want: true},
		{name: "opt-out/resource ignore", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationModeOptOut, want: false},
		{name: "opt-out/resource skip", resourceLabels: map[string]string{"zarf.dev/agent": "skip"}, mode: state.MutationModeOptOut, want: false},
		{name: "opt-out/namespace ignore", nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationModeOptOut, want: false},
		{name: "opt-out/namespace mutate", nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationModeOptOut, want: true},
		{name: "opt-out/resource mutate overrides namespace ignore", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationModeOptOut, want: true},
		{name: "opt-out/resource ignore overrides namespace mutate", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationModeOptOut, want: false},

		// Opt-in: same priority rules, but default is false
		{name: "opt-in/no labels", mode: state.MutationModeOptIn, want: false},
		{name: "opt-in/resource mutate", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationModeOptIn, want: true},
		{name: "opt-in/resource ignore", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationModeOptIn, want: false},
		{name: "opt-in/resource skip", resourceLabels: map[string]string{"zarf.dev/agent": "skip"}, mode: state.MutationModeOptIn, want: false},
		{name: "opt-in/namespace mutate", nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationModeOptIn, want: true},
		{name: "opt-in/namespace ignore", nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationModeOptIn, want: false},
		{name: "opt-in/resource mutate overrides namespace ignore", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationModeOptIn, want: true},
		{name: "opt-in/resource ignore overrides namespace mutate", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationModeOptIn, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := operations.ShouldMutate(tt.resourceLabels, tt.nsLabels, tt.mode)
			assert.Equal(t, tt.want, got)
		})
	}
}
