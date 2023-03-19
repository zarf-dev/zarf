// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// OrasRemote is a wrapper around the Oras remote repository that includes a progress bar for interactive feedback.
type OrasRemote struct {
	*remote.Repository
	context.Context
	*message.ProgressBar
}

// withScopes returns a context with the given scopes.
//
// This is needed for pushing to Docker Hub.
func withScopes(ref registry.Reference) context.Context {
	// For pushing to Docker Hub, we need to set the scope to the repository with pull+push actions, otherwise a 401 is returned
	scopes := []string{
		fmt.Sprintf("repository:%s:pull,push", ref.Repository),
	}
	return auth.WithScopes(context.Background(), scopes...)
}

// withAuthClient returns an auth client for the given reference.
//
// The credentials are pulled using Docker's default credential store.
func (o *OrasRemote) withAuthClient(ref registry.Reference) (*auth.Client, error) {
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return &auth.Client{}, err
	}
	if !cfg.ContainsAuth() {
		return &auth.Client{}, errors.New("no docker config file found, run 'zarf tools registry login --help'")
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = ref.Registry
	if key == "registry-1.docker.io" {
		// Docker stores its credentials under the following key, otherwise credentials use the registry URL
		key = "https://index.docker.io/v1/"
	}

	authConf, err := configs[0].GetCredentialsStore(key).Get(key)
	if err != nil {
		return &auth.Client{}, fmt.Errorf("unable to get credentials for %s: %w", key, err)
	}

	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = zarfconfig.CommonOptions.Insecure

	client := &auth.Client{
		Credential: auth.StaticCredential(ref.Registry, cred),
		Cache:      auth.NewCache(),
		Client: &http.Client{
			Transport: transport,
		},
	}
	client.SetUserAgent("zarf/" + zarfconfig.CLIVersion)

	client.Client.Transport = NewTransport(client.Client.Transport, o)

	return client, nil
}

// Transport is an http.RoundTripper that keeps track of the in-flight
// request and add hooks to report HTTP tracing events.
type Transport struct {
	Base       http.RoundTripper
	OrasRemote *OrasRemote
	Policy     func() retry.Policy
}

// NewTransport returns a custom transport that tracks an http.RoundTripper and an OrasRemote reference.
func NewTransport(base http.RoundTripper, o *OrasRemote) *Transport {
	return &Transport{
		Base:       base,
		OrasRemote: o,
	}
}

func (t *Transport) policy() retry.Policy {
	if t.Policy == nil {
		return retry.DefaultPolicy
	}
	return t.Policy()
}

type readCloser struct {
	io.Reader
	io.Closer
}

// Mirror of RoundTrip from retry
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	policy := t.policy()
	attempt := 0
	for {
		resp, respErr := t.roundTrip(req)
		duration, err := policy.Retry(attempt, resp, respErr)
		if err != nil {
			if respErr == nil {
				resp.Body.Close()
			}
			return nil, err
		}
		if duration < 0 {
			return resp, respErr
		}

		// rewind the body if possible
		if req.Body != nil {
			if req.GetBody == nil {
				// body can't be rewound, so we can't retry
				return resp, respErr
			}
			body, err := req.GetBody()
			if err != nil {
				// failed to rewind the body, so we can't retry
				return resp, respErr
			}
			req.Body = body
		}

		// close the response body if needed
		if respErr == nil {
			resp.Body.Close()
		}

		timer := time.NewTimer(duration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		attempt++
	}
}

// roundTrip calls base roundtrip while keeping track of the current request.
// This is currently only used to track the progress of publishes, not pulls.
func (t *Transport) roundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Method != http.MethodHead && req.Body != nil && t.OrasRemote.ProgressBar != nil {
		tee := io.TeeReader(req.Body, t.OrasRemote.ProgressBar)
		teeCloser := readCloser{tee, req.Body}
		req.Body = teeCloser
	}
	message.Debug(req.Method, req.URL, req.Context())

	resp, err = t.Base.RoundTrip(req)
	if err != nil {
		message.Debug("rt error:", err)
	}

	if resp != nil && req.Method == http.MethodHead && err == nil && t.OrasRemote.ProgressBar != nil {
		message.Debug(message.JSONValue(resp.Header))
		if resp.ContentLength > 0 {
			t.OrasRemote.ProgressBar.Add(int(resp.ContentLength))
		}
	}
	return resp, err
}

// NewOrasRemote returns an oras remote repository client and context for the given reference.
func NewOrasRemote(ref registry.Reference) (*OrasRemote, error) {
	o := &OrasRemote{}
	o.Context = withScopes(ref)
	// patch docker.io to registry-1.docker.io
	// this allows end users to use docker.io as an alias for registry-1.docker.io
	if ref.Registry == "docker.io" {
		ref.Registry = "registry-1.docker.io"
	}
	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return &OrasRemote{}, err
	}
	repo.PlainHTTP = zarfconfig.CommonOptions.Insecure
	authClient, err := o.withAuthClient(ref)
	if err != nil {
		return &OrasRemote{}, err
	}
	repo.Client = authClient
	o.Repository = repo
	return o, nil
}

// PrintLayerExists prints a success message to the console when a layer has been successfully published to a registry.
func PrintLayerExists(_ context.Context, desc ocispec.Descriptor) error {
	title := desc.Annotations[ocispec.AnnotationTitle]
	var format string
	if title != "" {
		format = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], First30last30(title))
	} else {
		format = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
	}
	message.Successf(format)
	return nil
}
