package packager

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/assert"
)

// TestValidateLastNonBreakingVersion verifies that Zarf validates the lastNonBreakingVersion of packages against the CLI version correctly.
func TestValidateLastNonBreakingVersion(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                   string
		cliVersion             string
		lastNonBreakingVersion string
		expectedErrorMessage   string
		expectedWarningMessage string
		returnError            bool
		throwWarning           bool
	}

	testCases := []testCase{
		{
			name:                   "CLI version less than lastNonBreakingVersion",
			cliVersion:             "v0.26.4",
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           true,
			expectedWarningMessage: fmt.Sprintf(
				lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
				"v0.26.4",
				"v0.27.0",
				"v0.27.0",
			),
		},
		{
			name:                   "invalid semantic version (CLI version)",
			cliVersion:             "invalidSemanticVersion",
			lastNonBreakingVersion: "v0.0.1",
			throwWarning:           false,
			returnError:            true,
			expectedErrorMessage:   "unable to parse Zarf CLI version",
		},
		{
			name:                   "invalid semantic version (lastNonBreakingVersion)",
			cliVersion:             "v0.0.1",
			lastNonBreakingVersion: "invalidSemanticVersion",
			throwWarning:           false,
			returnError:            true,
			expectedErrorMessage:   "unable to parse lastNonBreakingVersion",
		},
		{
			name:                   "CLI version greater than lastNonBreakingVersion",
			cliVersion:             "v0.28.2",
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           false,
		},
		{
			name:                   "CLI version equal to lastNonBreakingVersion",
			cliVersion:             "v0.27.0",
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           false,
		},
		{
			name:                   "empty lastNonBreakingVersion",
			cliVersion:             "this shouldn't get evaluated when the lastNonBreakingVersion is empty",
			lastNonBreakingVersion: "",
			returnError:            false,
			throwWarning:           false,
		},
		{
			name:                   "default CLI version in E2E tests",
			cliVersion:             "UnknownVersion", // This is used as a default version in the E2E tests
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			config.CLIVersion = testCase.cliVersion

			p := &Packager{
				cfg: &types.PackagerConfig{
					Pkg: types.ZarfPackage{
						Build: types.ZarfBuildData{
							LastNonBreakingVersion: testCase.lastNonBreakingVersion,
						},
					},
				},
			}

			err := p.validateLastNonBreakingVersion()

			switch {
			case testCase.returnError:
				assert.ErrorContains(t, err, testCase.expectedErrorMessage)
				assert.Empty(t, p.warnings, "Expected no warnings for test case: %s", testCase.name)
			case testCase.throwWarning:
				assert.Contains(t, p.warnings, testCase.expectedWarningMessage)
				assert.NoError(t, err, "Expected no error for test case: %s", testCase.name)
			default:
				assert.NoError(t, err, "Expected no error for test case: %s", testCase.name)
				assert.Empty(t, p.warnings, "Expected no warnings for test case: %s", testCase.name)
			}
		})
	}
}
