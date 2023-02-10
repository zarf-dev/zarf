// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	v1name "github.com/google/go-containerregistry/pkg/name"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// https://github.com/docker/docs/issues/8230
const OCILayerLimit = 127


// CtxWithScopes returns a context with the given scopes.
//
// This is needed for pushing to Docker Hub.
func CtxWithScopes(fullname string) context.Context {
	// For pushing to Docker Hub, we need to set the scope to the repository with pull+push actions, otherwise a 401 is returned
	scopes := []string{
		fmt.Sprintf("repository:%s:pull,push", fullname),
	}
	return auth.WithScopes(context.Background(), scopes...)
}

// AuthClient returns an auth client for the given reference.
//
// The credentials are pulled using Docker's default credential store.
func AuthClient(ref v1name.Reference) (*auth.Client, error) {
	// load default Docker config file
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return &auth.Client{}, err
	}
	if !cfg.ContainsAuth() {
		return &auth.Client{}, errors.New("no docker config file found, run 'docker login'")
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = ref.Context().RegistryStr()
	if key == "registry-1.docker.io" || key == "docker.io" {
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

	return &auth.Client{
		Credential: auth.StaticCredential(ref.Context().RegistryStr(), cred),
		Cache:      auth.NewCache(),
		// Gitlab auth fails if ForceAttemptOAuth2 is set to true
		// ForceAttemptOAuth2: true,
	}, nil
}

// isManifestUnsupported returns true if the error is an unsupported artifact manifest error.
//
// This function was copied verbatim from https://github.com/oras-project/oras/blob/main/cmd/oras/push.go
func IsManifestUnsupported(err error) bool {
	var errResp *errcode.ErrorResponse
	if !errors.As(err, &errResp) || errResp.StatusCode != http.StatusBadRequest {
		return false
	}

	var errCode errcode.Error
	if !errors.As(errResp, &errCode) {
		return false
	}

	// As of November 2022, ECR is known to return UNSUPPORTED error when
	// putting an OCI artifact manifest.
	switch errCode.Code {
	case errcode.ErrorCodeManifestInvalid, errcode.ErrorCodeUnsupported:
		return true
	}
	return false
}