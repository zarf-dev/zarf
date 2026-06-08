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
		mode           state.MutationPolicy
		want           bool
	}{
		// all: resource label takes priority
		{name: "all/no labels", mode: state.MutationPolicyAll, want: true},
		{name: "all/resource mutate", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationPolicyAll, want: true},
		{name: "all/resource ignore", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationPolicyAll, want: false},
		{name: "all/resource skip", resourceLabels: map[string]string{"zarf.dev/agent": "skip"}, mode: state.MutationPolicyAll, want: false},
		{name: "all/namespace ignore", nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationPolicyAll, want: false},
		{name: "all/namespace mutate", nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationPolicyAll, want: true},
		{name: "all/resource mutate overrides namespace ignore", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationPolicyAll, want: true},
		{name: "all/resource ignore overrides namespace mutate", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationPolicyAll, want: false},

		// labeled: same priority rules, but default is false
		{name: "labeled/no labels", mode: state.MutationPolicyLabeled, want: false},
		{name: "labeled/resource mutate", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationPolicyLabeled, want: true},
		{name: "labeled/resource ignore", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationPolicyLabeled, want: false},
		{name: "labeled/resource skip", resourceLabels: map[string]string{"zarf.dev/agent": "skip"}, mode: state.MutationPolicyLabeled, want: false},
		{name: "labeled/namespace mutate", nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationPolicyLabeled, want: true},
		{name: "labeled/namespace ignore", nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationPolicyLabeled, want: false},
		{name: "labeled/resource mutate overrides namespace ignore", resourceLabels: map[string]string{"zarf.dev/agent": "mutate"}, nsLabels: map[string]string{"zarf.dev/agent": "ignore"}, mode: state.MutationPolicyLabeled, want: true},
		{name: "labeled/resource ignore overrides namespace mutate", resourceLabels: map[string]string{"zarf.dev/agent": "ignore"}, nsLabels: map[string]string{"zarf.dev/agent": "mutate"}, mode: state.MutationPolicyLabeled, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := operations.ShouldMutate(tt.resourceLabels, tt.nsLabels, tt.mode)
			assert.Equal(t, tt.want, got)
		})
	}
}
