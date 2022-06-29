package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2eRemoteSgete(t *testing.T) {
	path := fmt.Sprintf("sget://defenseunicorns/zarf-hello-world:%s", e2e.arch)

	// Deploy the game
	output, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, output)
}
