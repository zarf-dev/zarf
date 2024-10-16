// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package logger implements a log/slog based logger in Zarf.
package logger

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		cfg  Config
	}{
		{
			name: "Empty level, format, and destination are ok",
			cfg:  Config{},
		},
		{
			name: "Default config is ok",
			cfg:  ConfigDefault(),
		},
		{
			name: "Debug logs are ok",
			cfg: Config{
				Level: Debug,
			},
		},
		{
			name: "Info logs are ok",
			cfg: Config{
				Level: Info,
			},
		},
		{
			name: "Warn logs are ok",
			cfg: Config{
				Level: Warn,
			},
		},
		{
			name: "Error logs are ok",
			cfg: Config{
				Level: Error,
			},
		},
		{
			name: "Text format is supported",
			cfg: Config{
				Format: FormatText,
			},
		},
		{
			name: "JSON format is supported",
			cfg: Config{
				Format: FormatJSON,
			},
		},
		{
			name: "FormatNone is supported to disable logs",
			cfg: Config{
				Format: FormatNone,
			},
		},
		{
			name: "DestinationNone is supported to disable logs",
			cfg: Config{
				Destination: DestinationNone,
			},
		},
		{
			name: "users can send logs to any io.Writer",
			cfg: Config{
				Destination: os.Stdout,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := New(tc.cfg)
			require.NoError(t, err)
			require.NotNil(t, res)
		})
	}
}

func Test_NewErrors(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		cfg  Config
	}{
		{
			name: "unsupported log level errors",
			cfg: Config{
				Level: 3,
			},
		},
		{
			name: "wildly unsupported log level errors",
			cfg: Config{
				Level: 42389412389213489,
			},
		},
		{
			name: "unsupported format errors",
			cfg: Config{
				Format: "foobar",
			},
		},
		{
			name: "wildly unsupported format errors",
			cfg: Config{
				Format: "^\\w+([-+.']\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$ lorem ipsum dolor sit amet 243897 )*&($#",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := New(tc.cfg)
			require.Error(t, err)
			require.Nil(t, res)
		})
	}
}

func Test_ParseLevel(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name   string
		s      string
		expect Level
	}{
		{
			name:   "can parse debug",
			s:      "debug",
			expect: Debug,
		},
		{
			name:   "can parse info",
			s:      "Info",
			expect: Info,
		},
		{
			name:   "can parse warn",
			s:      "warn",
			expect: Warn,
		},
		{
			name:   "can parse error",
			s:      "error",
			expect: Error,
		},
		{
			name:   "can handle uppercase",
			s:      "ERROR",
			expect: Error,
		},
		{
			name:   "can handle inconsistent uppercase",
			s:      "errOR",
			expect: Error,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseLevel(tc.s)
			require.NoError(t, err)
			require.Equal(t, tc.expect, res)
		})
	}
}

func Test_ParseLevelErrors(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		s    string
	}{
		{
			name: "errors out on unknown level",
			s:    "SUPER-DEBUG-10x-supremE",
		},
		{
			name: "is precise about character variations",
			s:    "érrør",
		},
		{
			name: "does not partial match level",
			s:    "error-info",
		},
		{
			name: "does not partial match level 2",
			s:    "info-error",
		},
		{
			name: "does not partial match level 3",
			s:    "info2",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseLevel(tc.s)
			require.Error(t, err)
		})
	}
}
