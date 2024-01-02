// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
)

func TestZarfComponent_IsRequired(t *testing.T) {
	tests := []struct {
		name      string
		component ZarfComponent
		want      bool
	}{
		{
			name: "Test when DeprecatedRequired is true and Optional is nil",
			component: ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           nil,
			},
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is true and Optional is false",
			component: ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           helpers.BoolPtr(false),
			},
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is true and Optional is true",
			component: ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(true),
				Optional:           helpers.BoolPtr(true),
			},
			// optional "wins" when defined
			want: false,
		},
		{
			name: "Test when DeprecatedRequired is false and Optional is nil",
			component: ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           nil,
			},
			want: false,
		},
		{
			name: "Test when DeprecatedRequired is false and Optional is false",
			component: ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           helpers.BoolPtr(false),
			},
			// optional "wins" when defined
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is false and Optional is true",
			component: ZarfComponent{
				DeprecatedRequired: helpers.BoolPtr(false),
				Optional:           helpers.BoolPtr(true),
			},
			// optional "wins" when defined
			want: false,
		},
		{
			name: "Test when DeprecatedRequired is nil and Optional is nil",
			component: ZarfComponent{
				DeprecatedRequired: nil,
				Optional:           nil,
			},
			// default is true (required: true || optional: false)
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is nil and Optional is false",
			component: ZarfComponent{
				DeprecatedRequired: nil,
				Optional:           helpers.BoolPtr(false),
			},
			// optional "wins" when defined
			want: true,
		},
		{
			name: "Test when DeprecatedRequired is nil and Optional is true",
			component: ZarfComponent{
				DeprecatedRequired: nil,
				Optional:           helpers.BoolPtr(true),
			},
			// optional "wins" when defined
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.component.IsRequired(); got != tt.want {
				t.Errorf("%q: ZarfComponent.IsRequired() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
