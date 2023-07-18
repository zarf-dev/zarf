package packager

import "testing"

func TestValidateMinimumCompatibleVersion(t *testing.T) {
	t.Parallel()

	/* Test Case 1:

	- Create a package tarball with the minimum compatible version (MCV) tagged in the build metadata

	- Load zarf package into memory

	- Set the CLI version variable to a version less than the MCV

	- Call the validateMinimumCompatibleVersion() function and assert that it returns an error

	*/

	/* Test Case 2:

	- Create a package tarball with the minimum compatible version (MCV) tagged in the build metadata

	- Load zarf package into memory

	- Set the CLI version variable to a version equal to or greater than the MCV

	- Call the validateMinimumCompatibleVersion() function and assert that it does not return an error

	*/
}
