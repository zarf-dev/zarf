// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

// PromptSigPassword prompts the user for the password to their private key
func PromptSigPassword() ([]byte, error) {
	var password string

	prompt := &survey.Password{
		Message: "Private key password (empty for no password): ",
	}
	return []byte(password), survey.AskOne(prompt, &password)
}

// PromptVariable prompts the user for a value for a variable
func PromptVariable(variable variables.InteractiveVariable) (value string, err error) {
	if variable.Description != "" {
		message.Question(variable.Description)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please provide a value for %q", variable.Name),
		Default: variable.Default,
	}

	return value, survey.AskOne(prompt, &value)
}
