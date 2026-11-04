// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCompression(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		format  VolumeCompression
		wantErr error
	}{
		{
			name:   "gzip",
			format: VolumeCompressionGzip,
		},
		{
			name:   "zstd",
			format: VolumeCompressionZstd,
		},
		{
			name:   "uncompressed",
			format: VolumeCompressionUncompressed,
		},
		{
			name:    "empty",
			format:  "",
			wantErr: ErrLayerCompression,
		},
		{
			name:    "invalid",
			format:  VolumeCompression("bogus"),
			wantErr: ErrLayerCompression,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCompression(tc.format)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidatePlatformOS(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		format  PlatformOS
		wantErr error
	}{
		{
			name:   "linux",
			format: PlatformOSLinux,
		},
		{
			name:   "windows",
			format: PlatformOSWindows,
		},
		{
			name:    "invalid",
			format:  PlatformOS("plan9"),
			wantErr: ErrPlatformOS,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePlatformOS(tc.format)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidatePlatformArch(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		format  PlatformArch
		wantErr error
	}{
		{
			name:   "amd64",
			format: PlatformArchAMD64,
		},
		{
			name:   "arm64",
			format: PlatformArchARM64,
		},
		{
			name:    "invalid",
			format:  PlatformArch("mips"),
			wantErr: ErrPlatformArch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePlatformArch(tc.format)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
