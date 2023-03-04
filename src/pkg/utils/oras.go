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
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

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
	// TODO:(@RAZZLE) https://github.com/oras-project/oras/blob/e8bc5acd9b7be47f2f9f387af6a963b14ae49eda/cmd/oras/internal/option/remote.go#L183

	client := &auth.Client{
		Credential: auth.StaticCredential(ref.Registry, cred),
		Cache:      auth.NewCache(),
		// Gitlab auth fails if ForceAttemptOAuth2 is set to true
		// ForceAttemptOAuth2: true,
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

func NewTransport(base http.RoundTripper, o *OrasRemote) *Transport {
	return &Transport{base, o}
}

type readCloser struct {
	io.Reader
	io.Closer
}

// RoundTrip calls base roundtrip while keeping track of the current request.
func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Body != nil && t.orasRemote.ProgressBar != nil {
		tee := io.TeeReader(req.Body, t.orasRemote.ProgressBar)
		teeCloser := readCloser{tee, req.Body}
		req.Body = teeCloser
	}

	resp, err = t.RoundTripper.RoundTrip(req)

	// do response things here

	return resp, err
}

// OrasRemote returns an oras remote repository client and context for the given reference.
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
