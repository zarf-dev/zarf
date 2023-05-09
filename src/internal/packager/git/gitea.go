// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	netHttp "net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// CreateTokenResponse is the response given from creating a token in Gitea
type CreateTokenResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Sha1           string `json:"sha1"`
	TokenLastEight string `json:"token_last_eight"`
}

// CreateReadOnlyUser uses the Gitea API to create a non-admin Zarf user.
func (g *Git) CreateReadOnlyUser() error {
	message.Debugf("git.CreateReadOnlyUser()")

	// Establish a git tunnel to send the repo
	tunnel, err := cluster.NewZarfTunnel()
	if err != nil {
		return err
	}
	err = tunnel.Connect(cluster.ZarfGit, false)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	tunnelURL := tunnel.HTTPEndpoint()

	// Determine if the read only user already exists
	getUserEndpoint := fmt.Sprintf("%s/api/v1/admin/users", tunnelURL)
	getUserRequest, _ := netHttp.NewRequest("GET", getUserEndpoint, nil)
	out, err := g.DoHTTPThings(getUserRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("GET %s:\n%s", getUserEndpoint, string(out))
	if err != nil {
		return err
	}

	hasReadOnlyUser := false
	var users []map[string]interface{}
	err = json.Unmarshal(out, &users)
	if err != nil {
		return err
	}

	for _, user := range users {
		if user["login"] == g.Server.PullUsername {
			hasReadOnlyUser = true
		}
	}

	if hasReadOnlyUser {
		// Update the existing user's password
		updateUserBody := map[string]interface{}{
			"login_name": g.Server.PullUsername,
			"password":   g.Server.PullPassword,
		}
		updateUserData, _ := json.Marshal(updateUserBody)
		updateUserEndpoint := fmt.Sprintf("%s/api/v1/admin/users/%s", tunnelURL, g.Server.PullUsername)
		updateUserRequest, _ := netHttp.NewRequest("PATCH", updateUserEndpoint, bytes.NewBuffer(updateUserData))
		out, err = g.DoHTTPThings(updateUserRequest, g.Server.PushUsername, g.Server.PushPassword)
		message.Debugf("PATCH %s:\n%s", updateUserEndpoint, string(out))
		return err
	}

	// Create json representation of the create-user request body
	createUserBody := map[string]interface{}{
		"username":             g.Server.PullUsername,
		"password":             g.Server.PullPassword,
		"email":                "zarf-reader@localhost.local",
		"must_change_password": false,
	}
	createUserData, err := json.Marshal(createUserBody)
	if err != nil {
		return err
	}

	// Send API request to create the user
	createUserEndpoint := fmt.Sprintf("%s/api/v1/admin/users", tunnelURL)
	createUserRequest, _ := netHttp.NewRequest("POST", createUserEndpoint, bytes.NewBuffer(createUserData))
	out, err = g.DoHTTPThings(createUserRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("POST %s:\n%s", createUserEndpoint, string(out))
	if err != nil {
		return err
	}

	// Make sure the user can't create their own repos or orgs
	updateUserBody := map[string]interface{}{
		"login_name":                g.Server.PullUsername,
		"max_repo_creation":         0,
		"allow_create_organization": false,
	}
	updateUserData, _ := json.Marshal(updateUserBody)
	updateUserEndpoint := fmt.Sprintf("%s/api/v1/admin/users/%s", tunnelURL, g.Server.PullUsername)
	updateUserRequest, _ := netHttp.NewRequest("PATCH", updateUserEndpoint, bytes.NewBuffer(updateUserData))
	out, err = g.DoHTTPThings(updateUserRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("PATCH %s:\n%s", updateUserEndpoint, string(out))
	return err
}

// CreatePackageRegistryToken uses the Gitea API to create a package registry token.
func (g *Git) CreatePackageRegistryToken() (CreateTokenResponse, error) {
	message.Debugf("git.CreatePackageRegistryToken()")

	// Establish a git tunnel to send the repo
	tunnel, err := cluster.NewZarfTunnel()
	if err != nil {
		return CreateTokenResponse{}, err
	}
	err = tunnel.Connect(cluster.ZarfGit, false)
	if err != nil {
		return CreateTokenResponse{}, err
	}
	defer tunnel.Close()

	tunnelURL := tunnel.Endpoint()

	// Determine if the package token already exists
	getTokensEndpoint := fmt.Sprintf("http://%s/api/v1/users/%s/tokens", tunnelURL, g.Server.PushUsername)
	getTokensRequest, _ := netHttp.NewRequest("GET", getTokensEndpoint, nil)
	out, err := g.DoHTTPThings(getTokensRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("GET %s:\n%s", getTokensEndpoint, string(out))
	if err != nil {
		return CreateTokenResponse{}, err
	}

	hasPackageToken := false
	var tokens []map[string]interface{}
	err = json.Unmarshal(out, &tokens)
	if err != nil {
		return CreateTokenResponse{}, err
	}

	for _, token := range tokens {
		if token["name"] == config.ZarfArtifactTokenName {
			hasPackageToken = true
		}
	}

	if hasPackageToken {
		// Delete the existing token to be replaced
		deleteTokensEndpoint := fmt.Sprintf("http://%s/api/v1/users/%s/tokens/%s", tunnelURL, g.Server.PushUsername, config.ZarfArtifactTokenName)
		deleteTokensRequest, _ := netHttp.NewRequest("DELETE", deleteTokensEndpoint, nil)
		out, err := g.DoHTTPThings(deleteTokensRequest, g.Server.PushUsername, g.Server.PushPassword)
		message.Debugf("DELETE %s:\n%s", deleteTokensEndpoint, string(out))
		if err != nil {
			return CreateTokenResponse{}, err
		}
	}

	createTokensEndpoint := fmt.Sprintf("http://%s/api/v1/users/%s/tokens", tunnelURL, g.Server.PushUsername)
	createTokensBody := map[string]interface{}{
		"name": config.ZarfArtifactTokenName,
	}
	createTokensData, _ := json.Marshal(createTokensBody)
	createTokensRequest, _ := netHttp.NewRequest("POST", createTokensEndpoint, bytes.NewBuffer(createTokensData))
	out, err = g.DoHTTPThings(createTokensRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("POST %s:\n%s", createTokensEndpoint, string(out))
	if err != nil {
		return CreateTokenResponse{}, err
	}

	createTokenResponse := CreateTokenResponse{}
	err = json.Unmarshal(out, &createTokenResponse)
	if err != nil {
		return CreateTokenResponse{}, err
	}

	return createTokenResponse, nil
}

// DoHTTPThings adds http request boilerplate and perform the request, checking for a successful response.
func (g *Git) DoHTTPThings(request *netHttp.Request, username, secret string) ([]byte, error) {
	message.Debugf("git.DoHttpThings()")

	// Prep the request with boilerplate
	client := &netHttp.Client{Timeout: time.Second * 20}
	request.SetBasicAuth(username, secret)
	request.Header.Add("accept", "application/json")
	request.Header.Add("Content-Type", "application/json")

	// Perform the request and get the response
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}
	responseBody, _ := io.ReadAll(response.Body)

	// If we get a 'bad' status code we will have no error, create a useful one to return
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err = fmt.Errorf("got status code of %d during http request with body of: %s", response.StatusCode, string(responseBody))
		return []byte{}, err
	}

	return responseBody, nil
}

func (g *Git) addReadOnlyUserToRepo(tunnelURL, repo string) error {
	message.Debugf("git.addReadOnlyUserToRepo()")

	// Add the readonly user to the repo
	addColabBody := map[string]string{
		"permission": "read",
	}
	addColabData, err := json.Marshal(addColabBody)
	if err != nil {
		return err
	}

	// Send API request to add a user as a read-only collaborator to a repo
	addColabEndpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/collaborators/%s", tunnelURL, g.Server.PushUsername, repo, g.Server.PullUsername)
	addColabRequest, _ := netHttp.NewRequest("PUT", addColabEndpoint, bytes.NewBuffer(addColabData))
	out, err := g.DoHTTPThings(addColabRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("PUT %s:\n%s", addColabEndpoint, string(out))
	return err
}
