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
		throwWarning             bool
	}

	testCases := []testCase{
		{
			name:                     "CLI version less than minimum compatible version",
			cliVersion:               "v0.26.4",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			throwWarning:             true,
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
			throwWarning:             false,
			returnError:              true,
			expectedErrorMessage:     "unable to parse Zarf CLI version",
		},
		{
			name:                     "invalid semantic version (minimum compatible version)",
			cliVersion:               "v0.0.1",
			minimumCompatibleVersion: "invalidSemanticVersion",
			throwWarning:             false,
			returnError:              true,
			expectedErrorMessage:     "unable to parse minimum compatible version",
		},
		{
			name:                     "CLI version greater than minimum compatible version",
			cliVersion:               "v0.28.2",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			throwWarning:             false,
		},
		{
			name:                     "CLI version equal to minimum compatible version",
			cliVersion:               "v0.27.0",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			throwWarning:             false,
		},
		{
			name:                     "empty minimum compatible version",
			cliVersion:               "this shouldn't get evaluated when the minimum compatible version string is empty",
			minimumCompatibleVersion: "",
			returnError:              false,
			throwWarning:             false,
		},
		{
			name:                     "default CLI version in E2E tests",
			cliVersion:               "UnknownVersion", // This is used as a default version in the E2E tests
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
			throwWarning:             false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		p := &Packager{}

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := p.validateMinimumCompatibleVersion(testCase.minimumCompatibleVersion, testCase.cliVersion)

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
