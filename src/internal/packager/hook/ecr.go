// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hook contains functions for operating hooks on the cluster.
package hook

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
)

func AuthToECR(registryURL string, ecrHook types.HookConfig) error {
	region := ecrHook.HookData["region"]

	ecrClient := ecrpublic.New(session.New(&aws.Config{Region: aws.String(region.(string))}))

	/* Auth into ECR */
	// TODO: @JPERRY Can I check if I'm already authed?
	authToken, err := ecrClient.GetAuthorizationToken(&ecrpublic.GetAuthorizationTokenInput{})
	if err != nil || authToken == nil || authToken.AuthorizationData == nil {
		return fmt.Errorf("unable to get the ECR authorization token: %w", err)
	}
	craneLogin := craneCmd.NewCmdAuthLogin()
	usernameErr := craneLogin.Flags().Set("username", "AWS")
	passwordErr := craneLogin.Flags().Set("password", *authToken.AuthorizationData.AuthorizationToken)
	if usernameErr != nil || passwordErr != nil {
		return fmt.Errorf("unable to set the ECR authorization credential")
	}

	// TODO: @JPERRY This is dumb.. crane only accepts strings that follow RFC 3986 URI syntax so I have to strip the Registry URL
	registryName := strings.Split(registryURL, "/")[0]
	err = craneLogin.RunE(craneLogin, []string{registryName})
	if err != nil {
		return fmt.Errorf("unable to login to the ECR registry: %w", err)
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
