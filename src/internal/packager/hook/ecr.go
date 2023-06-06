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
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	docker_types "github.com/docker/cli/cli/config/types"
)

func AuthToECR(ecrHook types.HookConfig) error {
	region := ecrHook.HookData["region"].(string)
	registryURL := ecrHook.HookData["registryURL"].(string)

	/* Auth into ECR */
	ecrClient := ecrpublic.New(session.New(&aws.Config{Region: aws.String(region)}))
	authToken, err := ecrClient.GetAuthorizationToken(&ecrpublic.GetAuthorizationTokenInput{})
	if err != nil || authToken == nil || authToken.AuthorizationData == nil {
		return fmt.Errorf("unable to get the ECR authorization token: %w", err)
	}

	data, err := b64.StdEncoding.DecodeString(*authToken.AuthorizationData.AuthorizationToken)
	if err != nil {
		return fmt.Errorf("unable to decode the ECR authorization token: %w", err)
	}
	username := "AWS"
	password := strings.Split(string(data), ":")[1]

	cfg, err := config.Load(config.Dir())
	if err != nil {
		return err
	}
	if !cfg.ContainsAuth() {
		return errors.New("no docker config file found, run 'zarf tools registry login --help'")
	}

	configs := []*configfile.ConfigFile{cfg}

	authConfig := docker_types.AuthConfig{Username: username, Password: password, ServerAddress: registryURL}
	err = configs[0].GetCredentialsStore(registryURL).Store(authConfig)
	if err != nil {
		return fmt.Errorf("unable to get credentials for %s: %w", registryURL, err)
	}

	return nil
}

func CreateTheECRRepos(ecrHook types.HookConfig, images []string) error {
	registryPrefix := ecrHook.HookData["repositoryPrefix"]
	region := ecrHook.HookData["region"]

	for _, image := range images {
		imageRef, err := transform.ParseImageRef(image)
		if err != nil {
			return fmt.Errorf("unable to parse the image ref: %w", err)
		}

		ecrClient := ecrpublic.New(session.New(&aws.Config{Region: aws.String(region.(string))}))

		repositoryName := registryPrefix.(string)
		if repositoryName != "" {
			repositoryName += "/"
		}
		repositoryName += imageRef.Path

		_, err = ecrClient.CreateRepository(&ecrpublic.CreateRepositoryInput{
			RepositoryName: aws.String(repositoryName)})
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				// Ignore errors if the repository already exists
				if aerr.Code() != ecrpublic.ErrCodeRepositoryAlreadyExistsException {
					return fmt.Errorf("unable to create the ECR repository: %w", err)
				}
			}
		}
	}

	return nil
}
