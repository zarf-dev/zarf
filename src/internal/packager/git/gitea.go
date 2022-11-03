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
	tunnel := cluster.NewZarfTunnel()
	tunnel.Connect(cluster.ZarfGit, false)
	defer tunnel.Close()

	tunnelUrl := tunnel.Endpoint()

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
	createUserEndpoint := fmt.Sprintf("http://%s/api/v1/admin/users", tunnelUrl)
	createUserRequest, _ := netHttp.NewRequest("POST", createUserEndpoint, bytes.NewBuffer(createUserData))
	out, err := g.DoHttpThings(createUserRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("POST %s:\n%s", createUserEndpoint, string(out))
	if err != nil {
		return err
	}

	// Make sure the user can't create their own repos or orgs
	updateUserBody := map[string]interface{}{
		"login_name":                g.Server.PushUsername,
		"max_repo_creation":         0,
		"allow_create_organization": false,
	}
	updateUserData, _ := json.Marshal(updateUserBody)
	updateUserEndpoint := fmt.Sprintf("http://%s/api/v1/admin/users/%s", tunnelUrl, g.Server.PullUsername)
	updateUserRequest, _ := netHttp.NewRequest("PATCH", updateUserEndpoint, bytes.NewBuffer(updateUserData))
	out, err = g.DoHttpThings(updateUserRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("PATCH %s:\n%s", updateUserEndpoint, string(out))
	return err
}

func (g *Git) addReadOnlyUserToRepo(tunnelUrl, repo string) error {
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
	addColabEndpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/collaborators/%s", tunnelUrl, g.Server.PushUsername, repo, g.Server.PullUsername)
	addColabRequest, _ := netHttp.NewRequest("PUT", addColabEndpoint, bytes.NewBuffer(addColabData))
	out, err := g.DoHttpThings(addColabRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("PUT %s:\n%s", addColabEndpoint, string(out))
	return err
}

// Add http request boilerplate and perform the request, checking for a successful response
func (g *Git) DoHttpThings(request *netHttp.Request, username, secret string) ([]byte, error) {
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
