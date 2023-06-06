// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hook contains functions for operating hooks on the cluster.
package hook

import (
	"errors"
	"fmt"
	"strings"

	b64 "encoding/base64"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	docker_types "github.com/docker/cli/cli/config/types"
)

const publicECRRegistryURL = "public.ecr.aws"

// AuthToECR fetches credentials for the ECR registry listed in the hook and saves them to the users local default docker config.json location
func AuthToECR(ecrHook types.HookConfig) error {
	region := ecrHook.HookData["region"].(string)
	registryURL := ecrHook.HookData["registryURL"].(string)

	var authToken string
	var err error
	if strings.Contains(registryURL, publicECRRegistryURL) {
		authToken, err = fetchAuthToPublicECR(registryURL, region)
	} else {
		authToken, err = fetchAuthToPrivateECR(registryURL, region)
	}
	if err != nil {
		return err
	}

	// Get the username and password from the auth token
	data, err := b64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return fmt.Errorf("unable to decode the ECR authorization token: %w", err)
	}
	username := "AWS"
	password := strings.Split(string(data), ":")[1]

	// Load the default docker.config file
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return err
	}
	if !cfg.ContainsAuth() {
		return errors.New("no docker config file found, run 'zarf tools registry login --help'")
	}

	// Save the credentials to the docker.config file
	configs := []*configfile.ConfigFile{cfg}
	authConfig := docker_types.AuthConfig{Username: username, Password: password, ServerAddress: registryURL}
	err = configs[0].GetCredentialsStore(registryURL).Store(authConfig)
	if err != nil {
		return fmt.Errorf("unable to get credentials for %s: %w", registryURL, err)
	}

	return nil
}

func fetchAuthToPublicECR(registryURL string, region string) (string, error) {
	ecrClient := ecrpublic.New(session.New(&aws.Config{Region: aws.String(region)}))
	authToken, err := ecrClient.GetAuthorizationToken(&ecrpublic.GetAuthorizationTokenInput{})
	if err != nil || authToken == nil || authToken.AuthorizationData == nil {
		return "", fmt.Errorf("unable to get the ECR authorization token: %w", err)
	}

	return *authToken.AuthorizationData.AuthorizationToken, nil
}

func fetchAuthToPrivateECR(registryURL string, region string) (string, error) {
	ecrClient := ecr.New(session.New(&aws.Config{Region: aws.String(region)}))
	authToken, err := ecrClient.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil || len(authToken.AuthorizationData) == 0 || authToken.AuthorizationData[0] == nil {
		return "", fmt.Errorf("unable to get the ECR authorization token: %w", err)
	}

	return *authToken.AuthorizationData[0].AuthorizationToken, err
}

// CreateTheECRRepos creates an ecr repository for each image provided
func CreateTheECRRepos(ecrHook types.HookConfig, images []string) error {
	registryPrefix := ecrHook.HookData["repositoryPrefix"].(string)
	region := ecrHook.HookData["region"].(string)
	registryURL := ecrHook.HookData["registryURL"].(string)

	if registryPrefix != "" {
		registryPrefix += "/"
	}

	// Create the ECR client
	var ecrClient *ecr.ECR
	var ecrPublicClient *ecrpublic.ECRPublic
	if strings.Contains(registryURL, publicECRRegistryURL) {
		ecrPublicClient = ecrpublic.New(session.New(&aws.Config{Region: aws.String(region)}))
	} else {
		ecrClient = ecr.New(session.New(&aws.Config{Region: aws.String(region)}))
	}

	for _, image := range images {
		// Parse the image ref
		imageRef, err := transform.ParseImageRef(image)
		if err != nil {
			return fmt.Errorf("unable to parse the image ref: %w", err)
		}

		repositoryName := registryPrefix + imageRef.Path
		if strings.Contains(registryURL, publicECRRegistryURL) {
			_, err = ecrPublicClient.CreateRepository(&ecrpublic.CreateRepositoryInput{RepositoryName: aws.String(repositoryName)})
		} else {
			_, err = ecrClient.CreateRepository(&ecr.CreateRepositoryInput{RepositoryName: aws.String(repositoryName)})
		}
		if aerr, ok := err.(awserr.Error); ok {
			// Ignore errors if the repository already exists
			if aerr.Code() != ecrpublic.ErrCodeRepositoryAlreadyExistsException {
				return fmt.Errorf("unable to create the ECR repository: %w", err)
			}
		}

	}

	return nil
}
