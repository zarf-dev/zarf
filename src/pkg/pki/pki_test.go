// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pki

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckForExpiredCert(t *testing.T) {
	// Test case 1: Certificate expires in 30 days (should be expiring soon).
	notAfterSoon := time.Duration(30 * 24 * time.Hour)
	pki, err := generatePKI("localhost", notAfterSoon)
	require.NoError(t, err)
	err = CheckForExpiredCert(context.Background(), pki)
	require.NoError(t, err)

	// Test case 2: Certificate expires in 90 days (should not be expiring soon).
	notAfterLater := time.Duration(90 * 24 * time.Hour)
	pki, err = generatePKI("localhost", notAfterLater)
	require.NoError(t, err)
	err = CheckForExpiredCert(context.Background(), pki)
	require.NoError(t, err)
}
