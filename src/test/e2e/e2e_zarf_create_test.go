package test

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2eZarfCreate(t *testing.T) {
	t.Log("E2E: Testing zarf create")
	e2e.setup(t)
	defer e2e.teardown(t)

	// run `zarf create` with a specified image cache location
	imageCachePath := "/tmp/.image_cache-location"
	e2e.cleanFiles(imageCachePath)

	output, err := e2e.execZarfCommand("package", "create", "--confirm", "--zarf-cache", imageCachePath)
	require.NoError(t, err, output)

	files, err := ioutil.ReadDir(imageCachePath)
	require.NoError(t, err, "Error when reading image cache path")
	assert.Greater(t, len(files), 1)

	e2e.cleanFiles(imageCachePath)
}
