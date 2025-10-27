// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package proxy provides tests for Zarf registry proxy mode.
package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryProxyInit(t *testing.T) {
	t.Log("Proxy Test: Zarf init with registry-proxy feature")

	// Run zarf init with registry proxy mode enabled
	stdOut, stdErr, err := e2e.Zarf(t, "init", "--features=registry-proxy=true", "--registry-mode=proxy", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the registry proxy TLS secrets were created
	_, _, err = e2e.Kubectl(t, "get", "secret", "-n", "zarf", "zarf-registry-server-tls")
	require.NoError(t, err, "zarf-registry-server-tls secret should exist")

	_, _, err = e2e.Kubectl(t, "get", "secret", "-n", "zarf", "zarf-registry-proxy-tls")
	require.NoError(t, err, "zarf-registry-proxy-tls secret should exist")
}
