// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// PromptSigPassword prompts the user for the password to their private key
func PromptSigPassword() ([]byte, error) {
	var password string

	prompt := &survey.Password{
		Message: "Private key password (empty for no password): ",
	}
	err := survey.AskOne(prompt, &password)
	if err != nil {
		return []byte{}, err
	}
	return []byte(password), nil
}

// PromptVariable prompts the user for a value for a variable
func PromptVariable(variable v1alpha1.InteractiveVariable) (string, error) {
	if variable.Description != "" {
		message.Question(variable.Description)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please provide a value for %q", variable.Name),
		Default: variable.Default,
	}

	var value string
	err := survey.AskOne(prompt, &value)
	if err != nil {
		return "", err
	}
	return value, nil
}
