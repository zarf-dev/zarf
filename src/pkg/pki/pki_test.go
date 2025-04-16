// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package pki

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

func TestCheckForExpiredCert(t *testing.T) {
	tests := []struct {
		name        string
		validFor    time.Duration
		expectedErr string
		expectedLog string
	}{
		{
			name:        "Certificate expires in 60 days",
			validFor:    60 * 24 * time.Hour,
			expectedErr: "",
			expectedLog: "the Zarf agent certificate is expirng soon, please run `zarf tools update-creds` to update the certificate",
		},
		{
			name:        "Certificate expires in 90 days",
			validFor:    90 * 24 * time.Hour,
			expectedErr: "",
		},
		{
			name:        "Certificate starts expired",
			validFor:    -1 * time.Second,
			expectedErr: "cert is expired, run `zarf tool update-creds agent`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cfg := logger.Config{
				Level:       logger.Info,
				Format:      logger.FormatConsole,
				Destination: buf,
			}
			l, err := logger.New(cfg)
			require.NoError(t, err)
			ctx := logger.WithContext(context.Background(), l)
			pki, err := generatePKI("localhost", tt.validFor)
			require.NoError(t, err)
			err = CheckForExpiredCert(ctx, pki)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			if tt.expectedLog != "" {
				require.Contains(t, buf.String(), tt.expectedLog)
			}
			require.NoError(t, err)
		})
	}
}
