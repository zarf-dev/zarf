// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"net/http"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/types"
)

// PullConfig is the configuration for pulling images.
type PullConfig struct {
	DestinationDirectory string

	ImageList []transform.Image

	Arch string

	RegistryOverrides map[string]string

	CacheDirectory string
}

const (
	DockerMediaTypeManifest     = "application/vnd.docker.distribution.manifest.v2+json"
	DockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

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
// based upon Zarf's global --insecure-skip-tls-verify (and --insecure) flags.
func WithGlobalInsecureFlag() []crane.Option {
	if config.CommonOptions.InsecureSkipTLSVerify {
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

func createPushOpts(cfg PushConfig) []crane.Option {
	opts := CommonOpts(cfg.Arch)
	opts = append(opts, WithPushAuth(cfg.RegInfo))

	defaultTransport := http.DefaultTransport.(*http.Transport).Clone()
	defaultTransport.TLSClientConfig.InsecureSkipVerify = config.CommonOptions.InsecureSkipTLSVerify
	// TODO (@WSTARR) This is set to match the TLSHandshakeTimeout to potentially mitigate effects of https://github.com/zarf-dev/zarf/issues/1444
	defaultTransport.ResponseHeaderTimeout = 10 * time.Second

	transport := helpers.NewTransport(defaultTransport, nil)

	opts = append(opts, crane.WithTransport(transport))

	return opts
}
