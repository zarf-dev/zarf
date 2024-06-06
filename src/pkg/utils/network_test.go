// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/defenseunicorns/pkg/helpers/v2"
)

func TestParseChecksum(t *testing.T) {
	t.Parallel()

	adr := "https://raw.githubusercontent.com/defenseunicorns/zarf/main/.adr-dir"
	sum := "930f4d5a191812e57b39bd60fca789ace07ec5acd36d63e1047604c8bdf998a3"

	tests := []struct {
		name        string
		url         string
		expectedURI string
		expectedSum string
	}{
		{
			name:        "url with checksum",
			url:         adr + "@" + sum,
			expectedURI: adr,
			expectedSum: sum,
		},
		{
			name:        "url with query parameters and checksum",
			url:         adr + "?foo=bar@" + sum,
			expectedURI: adr + "?foo=bar",
			expectedSum: sum,
		},
		{
			name:        "url with auth but without checksum",
			url:         "https://user:pass@hello.world?foo=bar",
			expectedURI: "https://user:pass@hello.world?foo=bar",
			expectedSum: "",
		},
		{
			name:        "url with auth and checksum",
			url:         "https://user:pass@hello.world?foo=bar@" + sum,
			expectedURI: "https://user:pass@hello.world?foo=bar",
			expectedSum: sum,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uri, checksum, err := parseChecksum(tt.url)
			require.NoError(t, err)
			require.Equal(t, tt.expectedURI, uri)
			require.Equal(t, tt.expectedSum, checksum)
		})
	}
}

func TestDownloadToFile(t *testing.T) {
	t.Parallel()

	// TODO: Explore replacing client transport instead of spinning up  http server.
	files := map[string]string{
		"README.md": "Hello World\n",
		".adr-dir":  "adr\n",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, fileName := path.Split(req.URL.Path)
		content, ok := files[fileName]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.Write([]byte(content))
	}))
	t.Cleanup(func() { srv.Close() })

	tests := []struct {
		name        string
		fileName    string
		queryParams string
		shasum      string
		expectedErr string
	}{
		{
			name:     "existing file",
			fileName: "README.md",
		},
		{
			name:        "non existing file",
			fileName:    "README.md.bad",
			expectedErr: "bad HTTP status: 404 Not Found",
		},
		{
			name:     "existing file with shasum",
			fileName: ".adr-dir",
			shasum:   "930f4d5a191812e57b39bd60fca789ace07ec5acd36d63e1047604c8bdf998a3",
		},
		{
			name:        "existing file with wrong shasum",
			fileName:    ".adr-dir",
			shasum:      "badsha",
			expectedErr: "expected badsha, got 930f4d5a191812e57b39bd60fca789ace07ec5acd36d63e1047604c8bdf998a3",
		},
		{
			name:        "existing file with shasum and query parameters",
			fileName:    ".adr-dir",
			queryParams: "foo=bar",
			shasum:      "930f4d5a191812e57b39bd60fca789ace07ec5acd36d63e1047604c8bdf998a3",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := fmt.Sprintf("%s/%s", srv.URL, tt.fileName)
			if tt.queryParams != "" {
				src = strings.Join([]string{src, tt.queryParams}, "?")
			}
			if tt.shasum != "" {
				src = strings.Join([]string{src, tt.shasum}, "@")
			}
			fmt.Println(src)
			dst := filepath.Join(t.TempDir(), tt.fileName)
			err := DownloadToFile(src, dst, "")
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.FileExists(t, dst)
			if tt.shasum == "" {
				return
			}
			check, err := helpers.GetSHA256OfFile(dst)
			require.NoError(t, err)
			require.Equal(t, tt.shasum, check)
		})
	}
}
