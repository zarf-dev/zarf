// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package checksum_test

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/checksum"
)

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
