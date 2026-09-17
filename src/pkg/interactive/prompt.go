// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// PromptVariable prompts the user for a value for a variable
func PromptVariable(ctx context.Context, variable api.InteractiveVariable) (string, error) {
	if variable.Description != "" {
		logger.From(ctx).Info(variable.Description)
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
