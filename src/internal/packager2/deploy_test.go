package packager2

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config/lang"
)

func TestValidateLastNonBreakingVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		cliVersion             string
		lastNonBreakingVersion string
		expectedErr            string
		expectedWarnings       []string
	}{
		{
			name:                   "CLI version less than last non breaking version",
			cliVersion:             "v0.26.4",
			lastNonBreakingVersion: "v0.27.0",
			expectedWarnings: []string{
				fmt.Sprintf(
					lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
					"v0.26.4",
					"v0.27.0",
					"v0.27.0",
				),
			},
		},
		{
			name:                   "invalid cli version",
			cliVersion:             "invalidSemanticVersion",
			lastNonBreakingVersion: "v0.0.1",
			expectedWarnings:       []string{fmt.Sprintf(lang.CmdPackageDeployInvalidCLIVersionWarn, "invalidSemanticVersion")},
		},
		{
			name:                   "invalid last non breaking version",
			cliVersion:             "v0.0.1",
			lastNonBreakingVersion: "invalidSemanticVersion",
			expectedErr:            "unable to parse last non breaking version",
		},
		{
			name:                   "CLI version greater than last non breaking version",
			cliVersion:             "v0.28.2",
			lastNonBreakingVersion: "v0.27.0",
		},
		{
			name:                   "CLI version equal to last non breaking version",
			cliVersion:             "v0.27.0",
			lastNonBreakingVersion: "v0.27.0",
		},
		{
			name:                   "empty last non breaking version",
			cliVersion:             "",
			lastNonBreakingVersion: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := validateLastNonBreakingVersion(tt.cliVersion, tt.lastNonBreakingVersion)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				require.Empty(t, warnings)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedWarnings, warnings)
		})
	}
}
