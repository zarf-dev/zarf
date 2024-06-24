// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"net/http"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// PullConfig is the configuration for pulling images.
type PullConfig struct {
	DestinationDirectory string

	ImageList []transform.Image

	Arch string

	RegistryOverrides map[string]string

	CacheDirectory string
}

// PushConfig is the configuration for pushing images.
type PushConfig struct {
	SourceDirectory string

	ImageList []transform.Image

	RegInfo types.RegistryInfo

	NoChecksum bool

	Arch string

	Retries int
}

// NoopOpt is a no-op option for crane.
func NoopOpt(*crane.Options) {}

// WithGlobalInsecureFlag returns an option for crane that configures insecure
// based upon Zarf's global --insecure flag.
func WithGlobalInsecureFlag() []crane.Option {
	if config.CommonOptions.Insecure {
		return []crane.Option{crane.Insecure}
	}
	// passing a nil option will cause panic
	return []crane.Option{NoopOpt}
}

// WithArchitecture sets the platform option for crane.
//
// This option is actually a slight mis-use of the platform option, as it is
// setting the architecture only and hard coding the OS to linux.
func WithArchitecture(arch string) crane.Option {
	return crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: arch})
}

// CommonOpts returns a set of common options for crane under Zarf.
func CommonOpts(arch string) []crane.Option {
	opts := WithGlobalInsecureFlag()
	opts = append(opts, WithArchitecture(arch))

	opts = append(opts,
		crane.WithUserAgent("zarf"),
		crane.WithNoClobber(true),
		crane.WithJobs(1),
	)
	return opts
}

// WithBasicAuth returns an option for crane that sets basic auth.
func WithBasicAuth(username, password string) crane.Option {
	return crane.WithAuth(authn.FromConfig(authn.AuthConfig{
		Username: username,
		Password: password,
	}))
}

// WithPullAuth returns an option for crane that sets pull auth from a given registry info.
func WithPullAuth(ri types.RegistryInfo) crane.Option {
	return WithBasicAuth(ri.PullUsername, ri.PullPassword)
}

// WithPushAuth returns an option for crane that sets push auth from a given registry info.
func WithPushAuth(ri types.RegistryInfo) crane.Option {
	return WithBasicAuth(ri.PushUsername, ri.PushPassword)
}

func createPushOpts(cfg PushConfig, pb *message.ProgressBar) []crane.Option {
	opts := CommonOpts(cfg.Arch)
	opts = append(opts, WithPushAuth(cfg.RegInfo))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = config.CommonOptions.Insecure
	// TODO (@WSTARR) This is set to match the TLSHandshakeTimeout to potentially mitigate effects of https://github.com/defenseunicorns/zarf/issues/1444
	transport.ResponseHeaderTimeout = 10 * time.Second

	transportWithProgressBar := helpers.NewTransport(transport, pb)

	opts = append(opts, crane.WithTransport(transportWithProgressBar))

	return opts
}
