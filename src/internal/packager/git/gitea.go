// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories
package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	netHttp "net/http"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// CreateReadOnlyUser uses the Gitea API to create a non-admin zarf user
func (g *Git) CreateReadOnlyUser() error {
	message.Debugf("git.CreateReadOnlyUser()")

	// Establish a git tunnel to send the repo
	tunnel, err := cluster.NewZarfTunnel()
	if err != nil {
		return err
	}
	tunnel.Connect(cluster.ZarfGit, false)
	defer tunnel.Close()

	tunnelURL := tunnel.Endpoint()

	// Determine if the read only user already exists
	getUserEndpoint := fmt.Sprintf("http://%s/api/v1/admin/users", tunnelURL)
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
		updateUserEndpoint := fmt.Sprintf("http://%s/api/v1/admin/users/%s", tunnelURL, g.Server.PullUsername)
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
	createUserEndpoint := fmt.Sprintf("http://%s/api/v1/admin/users", tunnelURL)
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
	updateUserEndpoint := fmt.Sprintf("http://%s/api/v1/admin/users/%s", tunnelURL, g.Server.PullUsername)
	updateUserRequest, _ := netHttp.NewRequest("PATCH", updateUserEndpoint, bytes.NewBuffer(updateUserData))
	out, err = g.DoHTTPThings(updateUserRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("PATCH %s:\n%s", updateUserEndpoint, string(out))
	return err
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

// DoHTTPThings adds http request boilerplate and perform the request, checking for a successful response
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
