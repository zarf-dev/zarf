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

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
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

	client.Client.Transport = NewTransport(client.Client.Transport, o)

	return client, nil
}

// Transport is an http.RoundTripper that keeps track of the in-flight
// request and add hooks to report HTTP tracing events.
type Transport struct {
	http.RoundTripper
	orasRemote *OrasRemote
}

// NewTransport returns a custom transport that tracks an http.RoundTripper and an OrasRemote reference.
func NewTransport(base http.RoundTripper, o *OrasRemote) *Transport {
	return &Transport{base, o}
}

type readCloser struct {
	io.Reader
	io.Closer
}

// RoundTrip calls base roundtrip while keeping track of the current request.
// This is currently only used to track the progress of publishes, not pulls.
func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Body != nil && t.orasRemote.ProgressBar != nil {
		tee := io.TeeReader(req.Body, t.orasRemote.ProgressBar)
		teeCloser := readCloser{tee, req.Body}
		req.Body = teeCloser
	}

	resp, err = t.RoundTripper.RoundTrip(req)

	if resp != nil && req.Body == nil && t.orasRemote.ProgressBar != nil && req.Method == http.MethodHead && resp.ContentLength > 0 {
		t.orasRemote.ProgressBar.Add(int(resp.ContentLength))
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
func PrintLayerExists(ctx context.Context, desc ocispec.Descriptor) error {
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
