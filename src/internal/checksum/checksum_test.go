// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package checksum_test

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/checksum"
)

const helloSHA256 = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

func TestGetSHA256Hash(t *testing.T) {
	t.Parallel()

	got, err := checksum.GetSHA256Hash(strings.NewReader("hello"))
	require.NoError(t, err)
	require.Equal(t, helloSHA256, got)

	readErr := errors.New("read failed")
	got, err = checksum.GetSHA256Hash(iotest.ErrReader(readErr))
	require.Empty(t, got)
	require.ErrorIs(t, err, readErr)
}

func TestGetSHA256OfFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "payload")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o600))
	got, err := checksum.GetSHA256OfFile(path)
	require.NoError(t, err)
	require.Equal(t, helloSHA256, got)

	got, err = checksum.GetSHA256OfFile(filepath.Join(t.TempDir(), "missing"))
	require.Empty(t, got)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestVerifyFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "payload")
	contents := []byte("package payload")
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	sum256 := sha256.Sum256(contents)
	sum512 := sha512.Sum512(contents)
	sha256Hex := hex.EncodeToString(sum256[:])
	sha512Hex := hex.EncodeToString(sum512[:])

	for _, tc := range []struct {
		algorithm api.ChecksumAlgorithm
		digest    string
	}{
		{api.ChecksumSHA256, sha256Hex},
		{api.ChecksumSHA512, sha512Hex},
	} {
		require.NoError(t, checksum.VerifyFile(path, tc.algorithm, tc.digest))
	}
	wrong256 := hex.EncodeToString(make([]byte, sha256.Size))
	require.EqualError(t, checksum.VerifyFile(path, api.ChecksumSHA256, wrong256),
		fmt.Sprintf("expected sha256 of %s to be %s, found %s", path, wrong256, sha256Hex))
	wrong512 := hex.EncodeToString(make([]byte, sha512.Size))
	require.EqualError(t, checksum.VerifyFile(path, api.ChecksumSHA512, wrong512),
		fmt.Sprintf("expected sha512 of %s to be %s, found %s", path, wrong512, sha512Hex))
	require.ErrorContains(t, checksum.VerifyFile(path, "sha384", sha256Hex), "unsupported checksum algorithm")
	require.ErrorContains(t, checksum.VerifyFile(path, api.ChecksumSHA256, "not-hex"), "invalid sha256 checksum \"not-hex\"")
	require.ErrorContains(t, checksum.VerifyFile(path, api.ChecksumSHA256, sha256Hex+"zz"), "invalid sha256 checksum")
}
