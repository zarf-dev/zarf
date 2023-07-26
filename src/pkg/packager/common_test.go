package packager

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/stretchr/testify/assert"
)

// TestValidateMinimumCompatibleVersion verifies that Zarf validates the minimum compatible version of packages against the CLI version correctly.
func TestValidateMinimumCompatibleVersion(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                     string
		cliVersion               string
		minimumCompatibleVersion string
		expectedErrorMessage     string
		expectedWarningMessage   string
		returnError              bool
		returnWarning            bool
	}

	testCases := []testCase{
		{
			name:                     "CLI version less than minimum compatible version",
			cliVersion:               "v0.26.4",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			returnWarning:            true,
			expectedWarningMessage: fmt.Sprintf(
				lang.CmdPackageDeployValidateMinCompatVersionWarn,
				"v0.26.4",
				"v0.27.0",
				"v0.27.0",
			),
		},
		{
			name:                     "invalid semantic version (CLI version)",
			cliVersion:               "invalidSemanticVersion",
			minimumCompatibleVersion: "v0.0.1",
			returnWarning:            false,
			returnError:              true,
			expectedErrorMessage:     "unable to parse Zarf CLI version",
		},
		{
			name:                     "invalid semantic version (minimum compatible version)",
			cliVersion:               "v0.0.1",
			minimumCompatibleVersion: "invalidSemanticVersion",
			returnWarning:            false,
			returnError:              true,
			expectedErrorMessage:     "unable to parse minimum compatible version",
		},
		{
			name:                     "CLI version greater than minimum compatible version",
			cliVersion:               "v0.28.2",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			returnWarning:            false,
		},
		{
			name:                     "CLI version equal to minimum compatible version",
			cliVersion:               "v0.27.0",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			returnWarning:            false,
		},
		{
			name:                     "empty minimum compatible version",
			cliVersion:               "this shouldn't get evaluated when the minimum compatible version string is empty",
			minimumCompatibleVersion: "",
			returnError:              false,
			returnWarning:            false,
		},
		{
			name:                     "default CLI version in E2E tests",
			cliVersion:               "UnknownVersion", // This is used as a default version in the E2E tests
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			returnWarning:            false,
		},
	}

	p := &Packager{}

	for _, testCase := range testCases {
		testCase := testCase // create a local copy of this loop variable

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			warning, err := p.validateMinimumCompatibleVersion(testCase.minimumCompatibleVersion, testCase.cliVersion)

			switch {
			case testCase.returnError:
				assert.ErrorContains(t, err, testCase.expectedErrorMessage)
				assert.Empty(t, warning)
			case testCase.returnWarning:
				assert.NoError(t, err, "Expected no error for test case: %s", testCase.name)
				assert.Equal(t, warning, testCase.expectedWarningMessage)
			default:
				assert.NoError(t, err, "Expected no error for test case: %s", testCase.name)
				assert.Empty(t, warning)
			}

		})
	}
}
