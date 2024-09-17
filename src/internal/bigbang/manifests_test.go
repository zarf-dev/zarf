// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bigbang contains the logic for installing Big Bang and Flux
package bigbang

// func TestManifestGitRepo(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		version     string
// 		repo        string
// 		expectedErr bool
// 		expectedAPI string
// 	}{
// 		{
// 			name:        "Valid version and repo",
// 			version:     "2.7.0",
// 			repo:        "https://github.com/bigbang/bigbang.git",
// 			expectedErr: false,
// 			expectedAPI: "source.toolkit.fluxcd.io/v1",
// 		},
// 		{
// 			name:        "Invalid version format",
// 			version:     "invalid-version",
// 			repo:        "https://github.com/bigbang/bigbang.git",
// 			expectedErr: true,
// 		},
// 		{
// 			name:        "Version that triggers update to v1 API",
// 			version:     "2.7.0",
// 			repo:        "https://github.com/bigbang/bigbang.git",
// 			expectedErr: false,
// 			expectedAPI: "source.toolkit.fluxcd.io/v1",
// 		},
// 		{
// 			name:        "Version that does not trigger update to v1 API",
// 			version:     "2.6.9",
// 			repo:        "https://github.com/bigbang/bigbang.git",
// 			expectedErr: false,
// 			expectedAPI: "source.toolkit.fluxcd.io/v1beta2",
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := manifestGitRepo(tt.version, tt.repo)
// 			if tt.expectedErr {
// 				require.Error(t, err)
// 			} else {
// 				require.NoError(t, err)
// 				require.Equal(t, tt.expectedAPI, result.APIVersion)
// 				require.Equal(t, tt.repo, result.Spec.URL)
// 				require.Equal(t, tt.version, result.Spec.Reference.Tag)
// 			}
// 		})
// 	}
// }
