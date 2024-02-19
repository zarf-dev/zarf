// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func Test_isRequired(t *testing.T) {
	tests := []struct {
		name             string
		component        types.ZarfComponent
		useRequiredLogic bool
		want             bool
	}{
		{
			name: "Test when DeprecatedRequired is true and Optional is nil",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           nil,
			},
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is true and Optional is false",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           helpers.BoolPtr(false),
			},
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is true and Optional is true",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           helpers.BoolPtr(true),
			},
			// optional "wins" when defined
			want: false,
		},
		{
			name: "Test when DeprecatedRequired is false and Optional is nil",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           nil,
			},
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is false and Optional is false",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           helpers.BoolPtr(false),
			},
			// optional "wins" when defined
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is false and Optional is true",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           helpers.BoolPtr(true),
			},
			// optional "wins" when defined
			want: false,
		},
		{
			name: "Test when DeprecatedRequired is nil and Optional is nil",
			component: types.ZarfComponent{
				DeprecatedRequired: nil,
				Optional:           nil,
			},
			// default is true (required: true || optional: false)
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is nil and Optional is false",
			component: types.ZarfComponent{
				DeprecatedRequired: nil,
				Optional:           helpers.BoolPtr(false),
			},
			// optional "wins" when defined
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is nil and Optional is true",
			component: types.ZarfComponent{
				DeprecatedRequired: nil,
				Optional:           helpers.BoolPtr(true),
			},
			// optional "wins" when defined
			want: false,
		},
		{
			name: "Test when DeprecatedRequired is true, Optional is true and useRequiredLogic is true",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           helpers.BoolPtr(true),
			},
			useRequiredLogic: true,
			want:             true,
		},
		{
			name: "Test when DeprecatedRequired is true, Optional is false and useRequiredLogic is false",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           helpers.BoolPtr(false),
			},
			useRequiredLogic: false,
			want:             true,
		},
		{
			name: "Test when DeprecatedRequired is false, Optional is true and useRequiredLogic is true",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           helpers.BoolPtr(true),
			},
			useRequiredLogic: true,
			want:             false,
		},
		{
			name: "Test when DeprecatedRequired is false, Optional is false and useRequiredLogic is false",
			component: types.ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           helpers.BoolPtr(false),
			},
			useRequiredLogic: false,
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRequired(tt.component, tt.useRequiredLogic)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_includedOrExcluded(t *testing.T) {
	tests := []struct {
		name                    string
		componentName           string
		requestedComponentNames []string
		wantState               selectState
		wantRequestedComponent  string
	}{
		{
			name:                    "Test when component is excluded",
			componentName:           "example",
			requestedComponentNames: []string{"-example"},
			wantState:               excluded,
			wantRequestedComponent:  "-example",
		},
		{
			name:                    "Test when component is included",
			componentName:           "example",
			requestedComponentNames: []string{"example"},
			wantState:               included,
			wantRequestedComponent:  "example",
		},
		{
			name:                    "Test when component is not included or excluded",
			componentName:           "example",
			requestedComponentNames: []string{"other"},
			wantState:               unknown,
			wantRequestedComponent:  "",
		},
		{
			name:                    "Test when component is excluded and included",
			componentName:           "example",
			requestedComponentNames: []string{"-example", "example"},
			wantState:               excluded,
			wantRequestedComponent:  "-example",
		},
		// interesting case, excluded wins
		{
			name:                    "Test when component is included and excluded",
			componentName:           "example",
			requestedComponentNames: []string{"example", "-example"},
			wantState:               excluded,
			wantRequestedComponent:  "-example",
		},
		{
			name:                    "Test when component is included via glob",
			componentName:           "example",
			requestedComponentNames: []string{"ex*"},
			wantState:               included,
			wantRequestedComponent:  "ex*",
		},
		{
			name:                    "Test when component is excluded via glob",
			componentName:           "example",
			requestedComponentNames: []string{"-ex*"},
			wantState:               excluded,
			wantRequestedComponent:  "-ex*",
		},
		{
			name:                    "Test when component is not found via glob",
			componentName:           "example",
			requestedComponentNames: []string{"other*"},
			wantState:               unknown,
			wantRequestedComponent:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotRequestedComponent := includedOrExcluded(tc.componentName, tc.requestedComponentNames)
			require.Equal(t, tc.wantState, gotState)
			require.Equal(t, tc.wantRequestedComponent, gotRequestedComponent)
		})
	}
}
