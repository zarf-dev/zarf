// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentOnlyVariable(t *testing.T) {
	t.Log("E2E: Component only.variable gating")
	t.Parallel()

	tmpdir := t.TempDir()
	src := filepath.Join("src", "test", "packages", "51-component-only-variable")

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", src, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-component-only-variable-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	allFiles := []string{
		"always-file.txt",
		"component-one-file.txt",
		"component-two-file.txt",
		"component-one-and-two-file.txt",
		"gated-by-constant-file.txt",
	}
	t.Cleanup(func() {
		e2e.CleanFiles(t, allFiles...)
	})

	type deployCase struct {
		name     string
		setFlags []string
		want     map[string]bool
	}

	cases := []deployCase{
		{
			name:     "defaults gate both off",
			setFlags: nil,
			want: map[string]bool{
				"always-file.txt":                true,
				"component-one-file.txt":         false,
				"component-two-file.txt":         false,
				"component-one-and-two-file.txt": false,
				"gated-by-constant-file.txt":     true,
			},
		},
		{
			name:     "component one only",
			setFlags: []string{"--set", "COMPONENT_ONE=true"},
			want: map[string]bool{
				"always-file.txt":                true,
				"component-one-file.txt":         true,
				"component-two-file.txt":         false,
				"component-one-and-two-file.txt": false,
				"gated-by-constant-file.txt":     true,
			},
		},
		{
			name:     "component two only",
			setFlags: []string{"--set", "COMPONENT_TWO=true"},
			want: map[string]bool{
				"always-file.txt":                true,
				"component-one-file.txt":         false,
				"component-two-file.txt":         true,
				"component-one-and-two-file.txt": false,
				"gated-by-constant-file.txt":     true,
			},
		},
		{
			name:     "both components",
			setFlags: []string{"--set", "COMPONENT_ONE=true", "--set", "COMPONENT_TWO=true"},
			want: map[string]bool{
				"always-file.txt":                true,
				"component-one-file.txt":         true,
				"component-two-file.txt":         true,
				"component-one-and-two-file.txt": true,
				"gated-by-constant-file.txt":     true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e2e.CleanFiles(t, allFiles...)

			args := append([]string{"package", "deploy", path, "--confirm"}, tc.setFlags...)
			stdOut, stdErr, err := e2e.Zarf(t, args...)
			require.NoError(t, err, stdOut, stdErr)

			for file, expectExists := range tc.want {
				if expectExists {
					require.FileExists(t, file, "expected %s to be deployed", file)
				} else {
					require.NoFileExists(t, file, "expected %s to be skipped", file)
				}
			}
		})
	}
}
