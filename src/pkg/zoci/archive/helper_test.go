// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package archive

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageRefToTar(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "repo with tag",
			ref:  "rpm:latest",
			want: "rpm-latest.tar",
		},
		{
			name: "repo without tag",
			ref:  "rpm",
			want: "rpm.tar",
		},
		{
			name: "registry with port and tag",
			ref:  "localhost:5000/rpm:latest",
			want: "localhost-5000_rpm-latest.tar",
		},
		{
			name: "registry host with namespace and tag",
			ref:  "example.com/ns/repo:tag",
			want: "example.com_ns_repo-tag.tar",
		},
		{
			name: "digest reference",
			ref:  "example.com/repo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85",
			want: "example.com_repo.tar",
		},
		{
			name: "tag and digest reference",
			ref:  "example.com/repo:tag@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85",
			want: "example.com_repo-tag.tar",
		},
		{
			name: "empty reference",
			ref:  "",
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, ImageRefToTar(tc.ref))
		})
	}
}

func TestValidateFileEndsWithTar(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		file      string
		expectErr error
	}{
		{
			name: "ends with .tar",
			file: "archive.tar",
		},
		{
			name: "nested path ending with .tar",
			file: "/tmp/out/rpm-latest.tar",
		},
		{
			name:      "no extension",
			file:      "archive",
			expectErr: ErrNotTarBall,
		},
		{
			name:      "wrong extension",
			file:      "archive.tar.gz",
			expectErr: ErrNotTarBall,
		},
		{
			name:      ".tar not at the end",
			file:      "archive.tarball",
			expectErr: ErrNotTarBall,
		},
		{
			name:      "empty file",
			file:      "",
			expectErr: ErrNotTarBall,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateFileEndsWithTar(tc.file)
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
