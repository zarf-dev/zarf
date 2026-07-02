package image

import (
	"context"
	"fmt"
)

// ErrPlatformMismatch is meant to be used when a provider has positively resolved the image but the image OS or
// architecture does not match with what was requested.
type ErrPlatformMismatch struct {
	ExpectedPlatform string
	Err              error
}

func (e *ErrPlatformMismatch) Error() string {
	if e.ExpectedPlatform == "" {
		return fmt.Sprintf("mismatched platform: %v", e.Err)
	}
	return fmt.Sprintf("mismatched platform (expected %v): %v", e.ExpectedPlatform, e.Err)
}

func (e *ErrPlatformMismatch) Unwrap() error {
	return e.Err
}

// Provider is an abstraction for any object that provides image objects (e.g. the docker daemon API, a tar file of
// an OCI image, podman varlink API, etc.).
type Provider interface {
	Name() string
	Provide(context.Context) (*Image, error)
}
