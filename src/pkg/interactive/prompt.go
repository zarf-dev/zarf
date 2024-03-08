// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
)

// PromptSigPassword prompts the user for the password to their private key
func PromptSigPassword() ([]byte, error) {
	var password string

	// If we're in interactive mode, prompt the user for the password to their private key
	if !config.CommonOptions.Confirm {
		prompt := &survey.Password{
			Message: "Private key password (empty for no password): ",
		}
		if err := survey.AskOne(prompt, &password); err != nil {
			return nil, fmt.Errorf("unable to get password for private key: %w", err)
		}
		return []byte(password), nil
	}

	// We are returning a nil error here because purposefully avoiding a password input is a valid use condition
	return nil, nil
}

// PromptVariable prompts the user for a value for a variable
func PromptVariable(variable variables.InteractiveVariable) (value string, err error) {

	if variable.Description != "" {
		message.Question(variable.Description)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please provide a value for \"%s\"", variable.Name),
		Default: variable.Default,
	}

	if err = survey.AskOne(prompt, &value); err != nil {
		return "", err
	}

	return value, nil
}
