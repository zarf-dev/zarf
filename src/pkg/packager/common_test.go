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
		returnError              bool
		expectedErrorMessage     string
	}

	testCases := []testCase{
		{
			name:                     "CLI version less than minimum compatible version",
			cliVersion:               "v0.26.4",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              true,
			expectedErrorMessage: fmt.Sprintf(
				lang.CmdPackageDeployValidateMinimumCompatibleVersionErr,
				"v0.26.4",
				"v0.27.0",
				"v0.27.0",
			),
		},
		{
			name:                     "invalid semantic version (CLI version)",
			cliVersion:               "invalidSemanticVersion",
			minimumCompatibleVersion: "v0.0.1",
			returnError:              true,
			expectedErrorMessage:     "unable to parse Zarf CLI version",
		},
		{
			name:                     "invalid semantic version (minimum compatible version)",
			cliVersion:               "v0.0.1",
			minimumCompatibleVersion: "invalidSemanticVersion",
			returnError:              true,
			expectedErrorMessage:     "unable to parse minimum compatible version",
		},
		{
			name:                     "CLI version greater than minimum compatible version",
			cliVersion:               "v0.28.2",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
		},
		{
			name:                     "CLI version equal to minimum compatible version",
			cliVersion:               "v0.27.0",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
		},
		{
			name:                     "empty minimum compatible version",
			cliVersion:               "this shouldn't get evaluated when the minimum compatible version string is empty",
			minimumCompatibleVersion: "",
			returnError:              false,
		},
		{
			name:                     "default CLI version in E2E tests",
			cliVersion:               "UnknownVersion", // This is used as a default version in the E2E tests
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
		},
	}

	p := &Packager{}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := p.validateMinimumCompatibleVersion(testCase.minimumCompatibleVersion, testCase.cliVersion)

			if testCase.returnError {
				assert.ErrorContains(t, err, testCase.expectedErrorMessage)
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", testCase.name)
			}
		})
	}
}
