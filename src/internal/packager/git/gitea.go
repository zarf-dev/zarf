// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	netHttp "net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CreateTokenResponse is the response given from creating a token in Gitea
type CreateTokenResponse struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Sha1           string `json:"sha1"`
	TokenLastEight string `json:"token_last_eight"`
}

// CreateReadOnlyUser uses the Gitea API to create a non-admin Zarf user.
func (g *Git) CreateReadOnlyUser(ctx context.Context) error {
	message.Debugf("git.CreateReadOnlyUser()")

	c, err := cluster.NewCluster()
	if err != nil {
		return err
	}

	// Establish a git tunnel to send the repo
	tunnel, err := c.NewTunnel(cluster.ZarfNamespaceName, k8s.SvcResource, cluster.ZarfGitServerName, "", 0, cluster.ZarfGitServerPort)
	if err != nil {
		return err
	}
	_, err = tunnel.Connect(ctx)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	tunnelURL := tunnel.HTTPEndpoint()

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

	var out []byte
	var statusCode int

	// Send API request to create the user
	createUserEndpoint := fmt.Sprintf("%s/api/v1/admin/users", tunnelURL)
	createUserRequest, _ := netHttp.NewRequest("POST", createUserEndpoint, bytes.NewBuffer(createUserData))
	err = tunnel.Wrap(func() error {
		out, statusCode, err = g.DoHTTPThings(createUserRequest, g.Server.PushUsername, g.Server.PushPassword)
		return err
	})
	message.Debugf("POST %s:\n%s", createUserEndpoint, string(out))
	if err != nil {
		if statusCode == 422 {
			message.Debugf("Read-only git user already exists.  Skipping...")
			return nil
		}

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
	err = tunnel.Wrap(func() error {
		out, _, err = g.DoHTTPThings(updateUserRequest, g.Server.PushUsername, g.Server.PushPassword)
		return err
	})
	message.Debugf("PATCH %s:\n%s", updateUserEndpoint, string(out))
	return err
}

// UpdateZarfGiteaUsers updates Zarf gitea users
func (g *Git) UpdateZarfGiteaUsers(ctx context.Context, oldState *types.ZarfState) error {

	//Update git read only user password
	err := g.UpdateGitUser(ctx, oldState.GitServer.PushPassword, g.Server.PullUsername, g.Server.PullPassword)
	if err != nil {
		return fmt.Errorf("unable to update gitea read only user password: %w", err)
	}

	// Update Git admin password
	err = g.UpdateGitUser(ctx, oldState.GitServer.PushPassword, g.Server.PushUsername, g.Server.PushPassword)
	if err != nil {
		return fmt.Errorf("unable to update gitea admin user password: %w", err)
	}
	return nil
}

// UpdateGitUser updates Zarf git server users
func (g *Git) UpdateGitUser(ctx context.Context, oldAdminPass string, username string, userpass string) error {
	message.Debugf("git.UpdateGitUser()")

	c, err := cluster.NewCluster()
	if err != nil {
		return err
	}
	// Establish a git tunnel to send the repo
	tunnel, err := c.NewTunnel(cluster.ZarfNamespaceName, k8s.SvcResource, cluster.ZarfGitServerName, "", 0, cluster.ZarfGitServerPort)
	if err != nil {
		return err
	}
	_, err = tunnel.Connect(ctx)
	if err != nil {
		return err
	}
	defer tunnel.Close()
	tunnelURL := tunnel.HTTPEndpoint()

	var out []byte

	// Update the existing user's password
	updateUserBody := map[string]interface{}{
		"login_name": username,
		"password":   userpass,
	}
	updateUserData, _ := json.Marshal(updateUserBody)
	updateUserEndpoint := fmt.Sprintf("%s/api/v1/admin/users/%s", tunnelURL, username)
	updateUserRequest, _ := netHttp.NewRequest("PATCH", updateUserEndpoint, bytes.NewBuffer(updateUserData))
	err = tunnel.Wrap(func() error {
		out, _, err = g.DoHTTPThings(updateUserRequest, g.Server.PushUsername, oldAdminPass)
		return err
	})
	message.Debugf("PATCH %s:\n%s", updateUserEndpoint, string(out))
	return err
}

// CreatePackageRegistryToken uses the Gitea API to create a package registry token.
func (g *Git) CreatePackageRegistryToken(ctx context.Context) (CreateTokenResponse, error) {
	message.Debugf("git.CreatePackageRegistryToken()")

	c, err := cluster.NewCluster()
	if err != nil {
		return CreateTokenResponse{}, err
	}

	// Establish a git tunnel to send the repo
	tunnel, err := c.NewTunnel(cluster.ZarfNamespaceName, k8s.SvcResource, cluster.ZarfGitServerName, "", 0, cluster.ZarfGitServerPort)
	if err != nil {
		return CreateTokenResponse{}, err
	}
	_, err = tunnel.Connect(ctx)
	if err != nil {
		return CreateTokenResponse{}, err
	}
	defer tunnel.Close()

	tunnelURL := tunnel.Endpoint()

	var out []byte

	// Determine if the package token already exists
	getTokensEndpoint := fmt.Sprintf("http://%s/api/v1/users/%s/tokens", tunnelURL, g.Server.PushUsername)
	getTokensRequest, _ := netHttp.NewRequest("GET", getTokensEndpoint, nil)
	err = tunnel.Wrap(func() error {
		out, _, err = g.DoHTTPThings(getTokensRequest, g.Server.PushUsername, g.Server.PushPassword)
		return err
	})
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
		err = tunnel.Wrap(func() error {
			out, _, err = g.DoHTTPThings(deleteTokensRequest, g.Server.PushUsername, g.Server.PushPassword)
			return err
		})
		message.Debugf("DELETE %s:\n%s", deleteTokensEndpoint, string(out))
		if err != nil {
			return CreateTokenResponse{}, err
		}
	}

	createTokensEndpoint := fmt.Sprintf("http://%s/api/v1/users/%s/tokens", tunnelURL, g.Server.PushUsername)
	createTokensBody := map[string]interface{}{
		"name":   config.ZarfArtifactTokenName,
		"scopes": []string{"read:user", "read:package", "write:package"},
	}
	createTokensData, _ := json.Marshal(createTokensBody)
	createTokensRequest, _ := netHttp.NewRequest("POST", createTokensEndpoint, bytes.NewBuffer(createTokensData))
	err = tunnel.Wrap(func() error {
		out, _, err = g.DoHTTPThings(createTokensRequest, g.Server.PushUsername, g.Server.PushPassword)
		return err
	})
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

// UpdateGiteaPVC updates the existing Gitea persistent volume claim and tells Gitea whether to create or not.
func UpdateGiteaPVC(ctx context.Context, shouldRollBack bool) (string, error) {
	c, err := cluster.NewCluster()
	if err != nil {
		return "false", err
	}

	pvcName := os.Getenv("ZARF_VAR_GIT_SERVER_EXISTING_PVC")
	groupKind := schema.GroupKind{
		Group: "",
		Kind:  "PersistentVolumeClaim",
	}
	labels := map[string]string{"app.kubernetes.io/managed-by": "Helm"}
	annotations := map[string]string{"meta.helm.sh/release-name": "zarf-gitea", "meta.helm.sh/release-namespace": "zarf"}

	if shouldRollBack {
		err = c.K8s.RemoveLabelsAndAnnotations(ctx, cluster.ZarfNamespaceName, pvcName, groupKind, labels, annotations)
		return "false", err
	}

	if pvcName == "data-zarf-gitea-0" {
		err = c.K8s.AddLabelsAndAnnotations(ctx, cluster.ZarfNamespaceName, pvcName, groupKind, labels, annotations)
		return "true", err
	}

	return "false", err
}

// DoHTTPThings adds http request boilerplate and perform the request, checking for a successful response.
func (g *Git) DoHTTPThings(request *netHttp.Request, username, secret string) ([]byte, int, error) {
	message.Debugf("git.DoHttpThings()")

	// Prep the request with boilerplate
	client := &netHttp.Client{Timeout: time.Second * 20}
	request.SetBasicAuth(username, secret)
	request.Header.Add("accept", "application/json")
	request.Header.Add("Content-Type", "application/json")

	// Perform the request and get the response
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, 0, err
	}
	responseBody, _ := io.ReadAll(response.Body)

	// If we get a 'bad' status code we will have no error, create a useful one to return
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err = fmt.Errorf("got status code of %d during http request with body of: %s", response.StatusCode, string(responseBody))
		return []byte{}, response.StatusCode, err
	}

	return responseBody, response.StatusCode, nil
}

func (g *Git) addReadOnlyUserToRepo(tunnelURL, repo string) error {
	message.Debugf("git.addReadOnlyUserToRepo()")

	// Add the readonly user to the repo
	addCollabBody := map[string]string{
		"permission": "read",
	}
	addCollabData, err := json.Marshal(addCollabBody)
	if err != nil {
		return err
	}

	// Send API request to add a user as a read-only collaborator to a repo
	addCollabEndpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/collaborators/%s", tunnelURL, g.Server.PushUsername, repo, g.Server.PullUsername)
	addCollabRequest, _ := netHttp.NewRequest("PUT", addCollabEndpoint, bytes.NewBuffer(addCollabData))
	out, _, err := g.DoHTTPThings(addCollabRequest, g.Server.PushUsername, g.Server.PushPassword)
	message.Debugf("PUT %s:\n%s", addCollabEndpoint, string(out))
	return err
}
