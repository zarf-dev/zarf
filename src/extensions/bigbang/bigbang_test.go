package bigbang

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRequiredBigBangVersions(t *testing.T) {
	// Support 1.54.0 and beyond
	err, vv := isValidVersion("1.54.0")
	assert.Equal(t, err, nil)
	assert.Equal(t, vv, true)

	// Do not support earlier than 1.54.0
	err, vv = isValidVersion("1.53.0")
	assert.Equal(t, err, nil)
	assert.Equal(t, vv, false)

	// Support for Big Bang release candidates
	err, vv = isValidVersion("1.57.0-rc.0")
	assert.Equal(t, err, nil)
	assert.Equal(t, vv, true)

	// Fail on non-semantic versions
	err, vv = isValidVersion("1.57")
	Expected := "No Major.Minor.Patch elements found"
	if err.Error() != Expected {
		t.Errorf("Error actual = %v, and Expected = %v.", err, Expected)
	}
	assert.Equal(t, vv, false)
}
