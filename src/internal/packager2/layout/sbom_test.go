package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestCreateImageSBOM(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	outputPath := t.TempDir()
	img := empty.Image
	b, err := createImageSBOM(ctx, t.TempDir(), outputPath, img, "docker.io/foo/bar:latest")
	require.NoError(t, err)
	require.NotEmpty(t, b)

	fileContent, err := os.ReadFile(filepath.Join(outputPath, "docker.io_foo_bar_latest.json"))
	require.NoError(t, err)
	require.Equal(t, fileContent, b)
}
