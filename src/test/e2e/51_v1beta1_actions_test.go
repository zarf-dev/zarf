// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestV1Beta1Actions(t *testing.T) {
	path := filepath.Join("src", "test", "packages", "51-v1beta1-actions")
	stdout, _, err := e2e.Zarf(t, "dev", "deploy", path)
	require.Error(t, err)
	require.Contains(t, stdout, "beta-before")
	require.Contains(t, stdout, "beta-before-success")
	require.Contains(t, stdout, "beta-failure-handled")
}
