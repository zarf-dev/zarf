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
	tests := []struct {
		name        string
		notAfter    time.Duration
		expectedErr string
	}{
		{
			name:        "Certificate expires in 30 days (should be expiring soon)",
			notAfter:    time.Duration(30 * 24 * time.Hour),
			expectedErr: "",
		},
		{
			name:        "Certificate expires in 90 days (should not be expiring soon)",
			notAfter:    time.Duration(90 * 24 * time.Hour),
			expectedErr: "",
		},
		{
			name:        "Certificate starts expired",
			notAfter:    time.Duration(-10 * time.Second),
			expectedErr: "cert is expired, run `zarf tool update-creds agent`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pki, err := generatePKI("localhost", tt.notAfter)
			require.NoError(t, err)
			err = CheckForExpiredCert(context.Background(), pki)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
