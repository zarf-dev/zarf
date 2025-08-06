// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package feature provides feature flags.
// NOTE: These functions rely on shared global state and therefore cannot be run in parallel.
package feature

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func setupFeatureState(t *testing.T, defaults []Feature, user []Feature) {
	t.Helper()

	// Cleanup first
	initFeatureState(t)

	// Set features if given
	if len(defaults) > 0 {
		defaultFeatures.Store(featuresToMap(defaults))
	}
	if len(user) > 0 {
		userFeatures.Store(featuresToMap(user))
	}
}

func initFeatureState(t *testing.T) {
	t.Helper()
	defaultFeatures.Store(map[Name]Feature{})
	userFeatures.Store(map[Name]Feature{})
}

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
			name:  "user enabled, default empty",
			fName: "foo",
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: true,
		},
		{
			name:  "default enabled, user empty",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: true,
		},
		{
			name:  "default disabled, user empty",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: false,
		},
		{
			name:  "user disabled, default empty",
			fName: "foo",
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: false,
		},
		{
			name:  "default enabled, user enabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: true,
		},
		{
			name:  "default disabled, user disabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: false,
		},
		{
			name:  "default disabled, user enabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: true,
		},
		{
			name:  "default enabled, user disabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: false,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// test setup
			setupFeatureState(t, tc.defaults, tc.user)
			defer initFeatureState(t)

			b := IsEnabled(tc.fName)
			require.Equal(t, tc.expect, b)
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
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
		},
		{
			name: "can set one disabled feature",
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
		},
		{
			name: "can set many features",
			user: []Feature{
				{Name: "foo", Enabled: true},
				{Name: "qux", Enabled: false},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: false},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure state is clean
			initFeatureState(t)
			defer initFeatureState(t)

			// Write
			err := Set(tc.user)
			require.NoError(t, err)

			// Read
			a := AllUser()
			require.Equal(t, a, featuresToMap(tc.user))
		})
	}
}

func TestSet_Errors(t *testing.T) {
	tt := []struct {
		name string
		user []Feature
	}{
		{
			name: "calling set multiple times errors",
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure state is clean
			initFeatureState(t)
			defer initFeatureState(t)

			// Write once
			require.NoError(t, Set(tc.user))
			// Write again
			require.Error(t, Set(tc.user))
		})
	}
}

func TestSetDefault(t *testing.T) {
	tt := []struct {
		name     string
		defaults []Feature
	}{
		{
			name: "can set no features",
		},
		{
			name: "can set one feature",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
		},
		{
			name: "can set one disabled feature",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
		},
		{
			name: "can set many features",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
				{Name: "qux", Enabled: false},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: false},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure state is clean
			initFeatureState(t)
			defer initFeatureState(t)

			// Write
			err := setDefault(tc.defaults)
			require.NoError(t, err)

			// Read
			a := AllDefault()
			require.Equal(t, a, featuresToMap(tc.defaults))
		})
	}
}

func TestSetDefault_Errors(t *testing.T) {
	tt := []struct {
		name string
		user []Feature
	}{
		{
			name: "calling Default multiple times errors",
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure state is clean
			initFeatureState(t)
			defer initFeatureState(t)

			// Write once
			require.NoError(t, setDefault(tc.user))
			// Write again
			require.Error(t, setDefault(tc.user))
		})
	}
}

func TestGet(t *testing.T) {
	tt := []struct {
		name      string
		defaultFs []Feature
		userFs    []Feature
		fName     Name
		expect    Feature
	}{
		{
			name:      "no error if default is empty",
			fName:     "foo",
			defaultFs: []Feature{},
			userFs: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: Feature{Name: "foo", Enabled: true},
		},
		{
			name:  "no error if user is empty",
			fName: "foo",
			defaultFs: []Feature{
				{Name: "foo", Enabled: true},
			},
			userFs: []Feature{},
			expect: Feature{Name: "foo", Enabled: true},
		},
		{
			name:  "user overrides default",
			fName: "foo",
			defaultFs: []Feature{
				{Name: "foo", Enabled: false},
			},
			userFs: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: Feature{Name: "foo", Enabled: true},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// State setup and teardown
			setupFeatureState(t, tc.defaultFs, tc.userFs)
			defer initFeatureState(t)

			// Write once
			f, err := Get(tc.fName)
			require.NoError(t, err)
			require.Equal(t, tc.expect, f)
		})
	}
}

func TestGet_Errors(t *testing.T) {
	tt := []struct {
		name      string
		defaultFs []Feature
		userFs    []Feature
		fName     Name
	}{
		{
			name:      "retrieving non-existant key errors",
			defaultFs: []Feature{},
			userFs:    []Feature{},
			fName:     "this-key-should-not-be-found",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// State setup and teardown
			setupFeatureState(t, tc.defaultFs, tc.userFs)
			defer initFeatureState(t)

			// Key should not be found
			_, err := Get(tc.fName)
			require.Error(t, err)
		})
	}
}

func TestGetUser(t *testing.T) {
	tt := []struct {
		name     string
		fName    Name
		features []Feature
		expect   Feature
	}{
		{
			name:  "can retrieve enabled feature",
			fName: "foo",
			features: []Feature{
				{Name: "foo", Enabled: true},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: false},
			},
			expect: Feature{Name: "foo", Enabled: true},
		},
		{
			name:  "can retrieve disabled feature",
			fName: "baz",
			features: []Feature{
				{Name: "foo", Enabled: false},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: false},
			},
			expect: Feature{Name: "baz", Enabled: false},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// State setup and teardown
			setupFeatureState(t, []Feature{}, tc.features)
			defer initFeatureState(t)

			// Write once
			f, err := GetUser(tc.fName)
			require.NoError(t, err)
			require.Equal(t, tc.expect, f)
		})
	}
}

func TestGetUser_Errors(t *testing.T) {
	tt := []struct {
		name     string
		features []Feature
		fName    Name
	}{
		{
			name:     "retrieving non-existant key errors",
			features: []Feature{},
			fName:    "this-key-should-not-be-found",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// State setup and teardown
			setupFeatureState(t, []Feature{}, tc.features)
			defer initFeatureState(t)

			// Key should not be found
			_, err := GetUser(tc.fName)
			require.Error(t, err)
		})
	}
}

func TestGetDefault(t *testing.T) {
	tt := []struct {
		name     string
		fName    Name
		features []Feature
		expect   Feature
	}{
		{
			name:  "can retrieve enabled feature",
			fName: "foo",
			features: []Feature{
				{Name: "foo", Enabled: true},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: false},
			},
			expect: Feature{Name: "foo", Enabled: true},
		},
		{
			name:  "can retrieve disabled feature",
			fName: "baz",
			features: []Feature{
				{Name: "foo", Enabled: false},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: false},
			},
			expect: Feature{Name: "baz", Enabled: false},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// State setup and teardown
			setupFeatureState(t, tc.features, []Feature{})
			defer initFeatureState(t)

			// Write once
			f, err := GetDefault(tc.fName)
			require.NoError(t, err)
			require.Equal(t, tc.expect, f)
		})
	}
}
func TestGetDefault_Errors(t *testing.T) {
	tt := []struct {
		name     string
		features []Feature
		fName    Name
	}{
		{
			name:     "retrieving non-existant key errors",
			features: []Feature{},
			fName:    "this-key-should-not-be-found",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// State setup and teardown
			setupFeatureState(t, []Feature{}, tc.features)
			defer initFeatureState(t)

			// Key should not be found
			_, err := GetDefault(tc.fName)
			require.Error(t, err)
		})
	}
}

func TestAll(t *testing.T) {
	tt := []struct {
		name     string
		user     []Feature
		defaults []Feature
		fName    Name
		expect   map[Mode]map[Name]Feature
	}{
		{
			name: "neither user nor default enabled",
			expect: map[Mode]map[Name]Feature{
				User:    {},
				Default: {},
			},
		},
		{
			name:  "user enabled, default empty",
			fName: "foo",
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: map[Mode]map[Name]Feature{
				User: {
					"foo": {Name: "foo", Enabled: true},
				},
				Default: {},
			},
		},
		{
			name:  "default enabled, user empty",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: map[Mode]map[Name]Feature{
				User: {},
				Default: {
					"foo": {Name: "foo", Enabled: true},
				},
			},
		},
		{
			name: "default disabled, user empty",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			fName: "foo",
			expect: map[Mode]map[Name]Feature{
				User: {},
				Default: {
					"foo": {Name: "foo", Enabled: false},
				},
			},
		},
		{
			name:  "user disabled, default empty",
			fName: "foo",
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: map[Mode]map[Name]Feature{
				User: {
					"foo": {Name: "foo", Enabled: false},
				},
				Default: {},
			},
		},
		{
			name:  "default enabled, user enabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: map[Mode]map[Name]Feature{
				Default: {
					"foo": {Name: "foo", Enabled: true},
				},
				User: {
					"foo": {Name: "foo", Enabled: true},
				},
			},
		},
		{
			name:  "default disabled, user disabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: map[Mode]map[Name]Feature{
				Default: {
					"foo": {Name: "foo", Enabled: false},
				},
				User: {
					"foo": {Name: "foo", Enabled: false},
				},
			},
		},
		{
			name:  "default disabled, user enabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: false},
			},
			user: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: map[Mode]map[Name]Feature{
				User: {
					"foo": {Name: "foo", Enabled: true},
				},
				Default: {
					"foo": {Name: "foo", Enabled: false},
				},
			},
		},
		{
			name:  "default enabled, user disabled",
			fName: "foo",
			defaults: []Feature{
				{Name: "foo", Enabled: true},
			},
			user: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: map[Mode]map[Name]Feature{
				Default: {
					"foo": {Name: "foo", Enabled: true},
				},
				User: {
					"foo": {Name: "foo", Enabled: false},
				},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// test setup and teardown
			setupFeatureState(t, tc.defaults, tc.user)
			defer initFeatureState(t)

			m := All()
			require.Equal(t, tc.expect, m)
		})
	}
}

func TestAllUser(t *testing.T) {
	tt := []struct {
		name     string
		features []Feature
		expect   map[Name]Feature
	}{
		{
			name:   "features empty",
			expect: map[Name]Feature{},
		},
		{
			name: "feature enabled",
			features: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: map[Name]Feature{
				"foo": {Name: "foo", Enabled: true},
			},
		},
		{
			name: "feature disabled",
			features: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: map[Name]Feature{
				"foo": {Name: "foo", Enabled: false},
			},
		},
		{
			name: "many features",
			features: []Feature{
				{Name: "foo", Enabled: false},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: true},
			},
			expect: map[Name]Feature{
				"foo": {Name: "foo", Enabled: false},
				"bar": {Name: "bar", Enabled: true},
				"baz": {Name: "baz", Enabled: true},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// test setup and teardown
			setupFeatureState(t, []Feature{}, tc.features)
			defer initFeatureState(t)

			m := AllUser()
			require.Equal(t, tc.expect, m)
		})
	}
}

func TestAllDefault(t *testing.T) {
	tt := []struct {
		name     string
		features []Feature
		expect   map[Name]Feature
	}{
		{
			name:   "features empty",
			expect: map[Name]Feature{},
		},
		{
			name: "feature enabled",
			features: []Feature{
				{Name: "foo", Enabled: true},
			},
			expect: map[Name]Feature{
				"foo": {Name: "foo", Enabled: true},
			},
		},
		{
			name: "feature disabled",
			features: []Feature{
				{Name: "foo", Enabled: false},
			},
			expect: map[Name]Feature{
				"foo": {Name: "foo", Enabled: false},
			},
		},
		{
			name: "many features",
			features: []Feature{
				{Name: "foo", Enabled: false},
				{Name: "bar", Enabled: true},
				{Name: "baz", Enabled: true},
			},
			expect: map[Name]Feature{
				"foo": {Name: "foo", Enabled: false},
				"bar": {Name: "bar", Enabled: true},
				"baz": {Name: "baz", Enabled: true},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// test setup and teardown
			setupFeatureState(t, tc.features, []Feature{})
			defer initFeatureState(t)

			m := AllDefault()
			require.Equal(t, tc.expect, m)
		})
	}
}
