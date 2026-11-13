//go:build !containers_image_openpgp

package containerstorage

import (
	"context"
	"fmt"

	"github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/image"
)

// Source is the image.Source string used to explicitly select this provider (e.g. "containers-storage:localhost/myimage:latest").
const Source image.Source = image.ContainersStorageSource

// errNotCompiledIn is returned when containers-storage support was not compiled into the binary. The real provider
// (see provider.go) is only built when the containers_image_openpgp build tag is set, since it pulls in the
// containers/image and containers/storage libraries.
var errNotCompiledIn = fmt.Errorf("containers-storage support is not compiled in (build with -tags containers_image_openpgp to enable)")

// NewProvider returns a stub provider used when containers-storage support is not compiled into the binary. It keeps
// the source name registered (so explicit "containers-storage:" references and source ordering behave consistently)
// while returning a clear error from Provide so that auto-resolution can continue to the next provider.
func NewProvider(_ *file.TempDirGenerator, _ string, _ *image.Platform) image.Provider {
	return &unsupportedProvider{}
}

type unsupportedProvider struct{}

func (p *unsupportedProvider) Name() string {
	return Source
}

func (p *unsupportedProvider) Provide(_ context.Context) (*image.Image, error) {
	return nil, errNotCompiledIn
}
