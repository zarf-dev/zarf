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
	}

	testCases := []testCase{
		{
			name:                     "Assert that a CLI version less than the minimum compatible version returns an error",
			cliVersion:               "v0.26.4",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              true,
		},
		{
			name:                     "Assert that a CLI version greater than the minimum compatible version does not return an error",
			cliVersion:               "v0.28.2",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
		},
		{
			name:                     "Assert that a CLI version equal to the minimum compatible version does not return an error",
			cliVersion:               "v0.27.0",
			minimumCompatibleVersion: "v0.27.0",
			returnError:              false,
		},
		{
			name:                     "Assert that an empty minimum compatible version string does not return an error",
			cliVersion:               "this shouldn't get evaluated when the minimum compatible version string is empty",
			minimumCompatibleVersion: "",
			returnError:              false,
		},
		{
			name:                     "Assert that the default CLI version string used in the E2E tests does not return an error",
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
				expectedErrorMessage := fmt.Sprintf(lang.CmdPackageDeployValidateMinimumCompatibleVersionErr, testCase.cliVersion, testCase.minimumCompatibleVersion, testCase.minimumCompatibleVersion)
				assert.ErrorContains(t, err, expectedErrorMessage)
			} else {
				assert.NoError(t, err, "Expected no error for test case: %s", testCase.name)
			}
		})
	}
}
