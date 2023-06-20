// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hook contains functions for operating hooks on the cluster.
package hook

import (
	"fmt"
	"strings"

	b64 "encoding/base64"
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types/hooks"
	docker_types "github.com/docker/cli/cli/config/types"
)

const PublicECRRegistryURL = "public.ecr.aws"
const PrivateECRRegistryURL = "amazonaws.com"

// ECRHookData contains the data for the ECR hook
type ECRHookData struct {
	Region         string `json:"region" jsonschema:"description=AWS region of the ECR registry"`
	RegistryURL    string `json:"registryURL" jsonschema:"description=URL of the ECR registry"`
	RegistryPrefix string `json:"repositoryPrefix" jsonschema:"description=Prefix of the ECR registry"`
}

// NewECRHookData creates a new ECRHookData struct with the data from hook data map
func NewECRHookData(hookData map[string]interface{}) (ECRHookData, error) {
	ecrHookData := ECRHookData{}
	hookDataBytes, err := json.Marshal(hookData)
	if err != nil {
		return ecrHookData, err
	}

	err = json.Unmarshal(hookDataBytes, &ecrHookData)
	if err != nil {
		return ecrHookData, err
	}

	return ecrHookData, err
}

// AuthToECR fetches credentials for the ECR registry listed in the hook and saves them
// to the users local default docker config.json location
func AuthToECR(ecrHook hooks.HookConfig) error {
	ecrHookData, err := NewECRHookData(ecrHook.HookData)
	if err != nil {
		return fmt.Errorf("unable to parse ecr hook data: %w", err)
	}

	var authToken string
	if strings.Contains(ecrHookData.RegistryURL, PublicECRRegistryURL) {
		authToken, err = fetchAuthToPublicECR(ecrHookData.Region)
	} else {
		authToken, err = fetchAuthToPrivateECR(ecrHookData.Region)
	}
	if err != nil {
		return err
	}

	// Get the username and password from the auth token
	// NOTE: The auth token is base64 encoded and contains the {USERNAME}:{PASSWORD}
	data, err := b64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return fmt.Errorf("unable to decode the ECR authorization token: %w", err)
	}
	username := strings.Split(string(data), ":")[0]
	password := strings.Split(string(data), ":")[1]
	authConfig := docker_types.AuthConfig{Username: username, Password: password, ServerAddress: ecrHookData.RegistryURL}

	// Save the auth config to the users docker config.json
	return utils.SaveDockerCredential(ecrHookData.RegistryURL, authConfig)
}

// fetchAuthToPublicECR uses the ECR public client to fetch a 12 hour auth token
func fetchAuthToPublicECR(region string) (string, error) {
	ecrClient := ecrpublic.New(session.New(&aws.Config{Region: aws.String(region)}))
	authToken, err := ecrClient.GetAuthorizationToken(&ecrpublic.GetAuthorizationTokenInput{})
	if err != nil || authToken == nil || authToken.AuthorizationData == nil {
		return "", fmt.Errorf("unable to get the ECR authorization token: %w", err)
	}

	return *authToken.AuthorizationData.AuthorizationToken, nil
}

// fetchAuthToPrivateECR uses the ECR private client to fetch a 12 hour auth token
// NOTE: The ECR private client has a slightly different API than the public client and returns a list of authData
//
//	The AWS docs say the ReistryIDs list is deprecated and to just use the first element in the list
func fetchAuthToPrivateECR(region string) (string, error) {
	ecrClient := ecr.New(session.New(&aws.Config{Region: aws.String(region)}))
	authToken, err := ecrClient.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil || len(authToken.AuthorizationData) == 0 || authToken.AuthorizationData[0] == nil {
		return "", fmt.Errorf("unable to get the ECR authorization token: %w", err)
	}

	return *authToken.AuthorizationData[0].AuthorizationToken, err
}

// CreateTheECRRepos creates an ecr repository for each image provided
func CreateTheECRRepos(ecrHook hooks.HookConfig, images []string) error {
	ecrHookData, err := NewECRHookData(ecrHook.HookData)
	if err != nil {
		return fmt.Errorf("unable to parse ecr hook data: %w", err)
	}

	// If a prefix was provided, add a trailing slash to it if it doesn't already have one
	registryPrefix := ecrHookData.RegistryPrefix
	if ecrHookData.RegistryPrefix != "" && !strings.HasSuffix(ecrHookData.RegistryPrefix, "/") {
		registryPrefix += "/"
	}

	// Create the ECR client
	var ecrClient *ecr.ECR
	var ecrPublicClient *ecrpublic.ECRPublic
	if strings.Contains(ecrHookData.RegistryURL, PublicECRRegistryURL) {
		ecrPublicClient = ecrpublic.New(session.New(&aws.Config{Region: aws.String(ecrHookData.Region)}))
	} else {
		ecrClient = ecr.New(session.New(&aws.Config{Region: aws.String(ecrHookData.Region)}))
	}

	for _, image := range images {
		// Parse the image ref
		imageRef, err := transform.ParseImageRef(image)
		if err != nil {
			return fmt.Errorf("unable to parse the image ref: %w", err)
		}

		repositoryName := registryPrefix + imageRef.Path
		if strings.Contains(ecrHookData.RegistryURL, PublicECRRegistryURL) {
			createRepositoryInput := &ecrpublic.CreateRepositoryInput{RepositoryName: aws.String(repositoryName)}
			_, err = ecrPublicClient.CreateRepository(createRepositoryInput)
		} else {
			createRepositoryInput := &ecr.CreateRepositoryInput{RepositoryName: aws.String(repositoryName)}
			_, err = ecrClient.CreateRepository(createRepositoryInput)
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
