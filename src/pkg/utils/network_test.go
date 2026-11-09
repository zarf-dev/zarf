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
	"github.com/zarf-dev/zarf/src/internal/checksum"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestParseChecksum(t *testing.T) {
	t.Parallel()

	fileURL := "https://example.com/.test-file"
	sum := "f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2"

	tests := []struct {
		name        string
		url         string
		expectedURI string
		expectedSum string
	}{
		{
			name:        "url with checksum",
			url:         fileURL + "@" + sum,
			expectedURI: fileURL,
			expectedSum: sum,
		},
		{
			name:        "url with query parameters and checksum",
			url:         fileURL + "?foo=bar@" + sum,
			expectedURI: fileURL + "?foo=bar",
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
		"README.md":  "Hello World\n",
		".test-file": "test\n",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, fileName := path.Split(req.URL.Path)
		content, ok := files[fileName]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		//nolint:errcheck // ignore
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
			fileName: ".test-file",
			shasum:   "f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
		},
		{
			name:        "existing file with wrong shasum",
			fileName:    ".test-file",
			shasum:      "badsha",
			expectedErr: "expected badsha, got f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
		},
		{
			name:        "existing file with shasum and query parameters",
			fileName:    ".test-file",
			queryParams: "foo=bar",
			shasum:      "f2ca1bb6c7e907d06dafe4687e579fce76b37e4e93b7605022da52e6ccc26fd2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := fmt.Sprintf("%s/%s", srv.URL, tt.fileName)
			if tt.queryParams != "" {
				src = strings.Join([]string{src, tt.queryParams}, "?")
			}
			if tt.shasum != "" {
				src = strings.Join([]string{src, tt.shasum}, "@")
			}
			dst := filepath.Join(t.TempDir(), tt.fileName)
			err := DownloadToFile(testutil.TestContext(t), src, dst)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.FileExists(t, dst)
			if tt.shasum == "" {
				return
			}
			check, err := checksum.GetSHA256OfFile(dst)
			require.NoError(t, err)
			require.Equal(t, tt.shasum, check)
		})
	}
}
