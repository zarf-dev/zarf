// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package feature provides feature flags.
package feature

import (
	"testing"
)

func TestIsEnabled(t *testing.T) {
	tt := []struct {
		name     string
		user     []Feature
		defaults []Feature
		fName    Name
		expect   bool
	}{
		{
			name:   "neither user nor default enabled",
			expect: false,
		},
		{
			name: "user enabled, default empty",
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			fName:  "foo",
			expect: true,
		},
		{
			name: "defaults enabled, user empty",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			fName:  "foo",
			expect: true,
		},
		{
			name: "defaults enabled, user enabled",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			fName:  "foo",
			expect: true,
		},
		{
			name: "defaults disabled, user enabled",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			fName:  "foo",
			expect: true,
		},
		{
			name: "defaults enabled, user disabled",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			fName:  "foo",
			expect: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// TODO:
			// Preload atoms
			// Run tests
			// Clear atoms
			t.Skip()
		})
	}
}

func TestSet(t *testing.T) {
	tt := []struct {
		name string
		user []Feature
	}{
		{
			name: "can set no features",
		},
		{
			name: "can set one feature",
		},
		{
			name: "can set many features",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// require.NoError(t, )
			t.Skip()
		})
	}
}

func TestSet_Errors(t *testing.T) {
	tt := []struct {
		name   string
		user   []Feature
		expect error
	}{
		{
			name: "setting one feature multiple times errors",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// require.NoError(t, )
			t.Skip()
		})
	}
}

func TestSetDefault(t *testing.T) {
	tt := []struct {
		name     string
		user     []Feature
		defaults []Feature
	}{
		{},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Skip()
		})
	}
}

func TestGet(t *testing.T) {
	t.Skip()
}

func TestGetUser(t *testing.T) {
	t.Skip()
}

func TestGetDefault(t *testing.T) {
	t.Skip()
}

func TestAll(t *testing.T) {
	t.Skip()
}

func TestAllUser(t *testing.T) {
	t.Skip()
}

func TestAllDefault(t *testing.T) {
	t.Skip()
}
