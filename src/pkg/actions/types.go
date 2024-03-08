// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package actions contains functions for running commands and tasks
package actions

import (
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
)

// ActionSet is a set of actions to run during an operation
type ActionSet struct {
	Defaults  ActionDefaults `json:"defaults,omitempty" jsonschema:"description=Default configuration for all actions in this set"`
	Before    []Action       `json:"before,omitempty" jsonschema:"description=Actions to run at the start of an operation"`
	After     []Action       `json:"after,omitempty" jsonschema:"description=Actions to run at the end of an operation"`
	OnSuccess []Action       `json:"onSuccess,omitempty" jsonschema:"description=Actions to run if all operations succeed"`
	OnFailure []Action       `json:"onFailure,omitempty" jsonschema:"description=Actions to run if all operations fail"`
}

// ActionDefaults sets the default configs for child actions
type ActionDefaults struct {
	Mute            bool           `json:"mute,omitempty" jsonschema:"description=Hide the output of commands during execution (default false)"`
	MaxTotalSeconds int            `json:"maxTotalSeconds,omitempty" jsonschema:"description=Default timeout in seconds for commands (default to 0, no timeout)"`
	MaxRetries      int            `json:"maxRetries,omitempty" jsonschema:"description=Retry commands given number of times if they fail (default 0)"`
	Dir             string         `json:"dir,omitempty" jsonschema:"description=Working directory for commands (default CWD)"`
	Env             []string       `json:"env,omitempty" jsonschema:"description=Additional environment variables for commands"`
	Shell           exec.ExecShell `json:"shell,omitempty" jsonschema:"description=(cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems"`
}

// Action represents a single action to run
type Action struct {
	Mute                  *bool               `json:"mute,omitempty" jsonschema:"description=Hide the output of the command during package deployment (default false)"`
	MaxTotalSeconds       *int                `json:"maxTotalSeconds,omitempty" jsonschema:"description=Timeout in seconds for the command (default to 0, no timeout for cmd actions and 300, 5 minutes for wait actions)"`
	MaxRetries            *int                `json:"maxRetries,omitempty" jsonschema:"description=Retry the command if it fails up to given number of times (default 0)"`
	Dir                   *string             `json:"dir,omitempty" jsonschema:"description=The working directory to run the command in (default is CWD)"`
	Env                   []string            `json:"env,omitempty" jsonschema:"description=Additional environment variables to set for the command"`
	Cmd                   string              `json:"cmd,omitempty" jsonschema:"description=The command to run. Must specify either cmd or wait for the action to do anything."`
	Shell                 *exec.ExecShell     `json:"shell,omitempty" jsonschema:"description=(cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems"`
	DeprecatedSetVariable string              `json:"setVariable,omitempty" jsonschema:"description=[Deprecated] (replaced by setVariables) (onDeploy/cmd only) The name of a variable to update with the output of the command. This variable will be available to all remaining actions and components in the package. This will be removed in Zarf v1.0.0,pattern=^[A-Z0-9_]+$"`
	SetVariables          []ActionSetVariable `json:"setVariables,omitempty" jsonschema:"description=(onDeploy/cmd only) An array of variables to update with the output of the command. These variables will be available to all remaining actions and components in the package."`
	Description           string              `json:"description,omitempty" jsonschema:"description=Description of the action to be displayed during package execution instead of the command"`
	Wait                  *ActionWait         `json:"wait,omitempty" jsonschema:"description=Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info."`
}

// ActionSetVariable represents a variable that is to be set via an action
type ActionSetVariable struct {
	Name       string                 `json:"name" jsonschema:"description=The name to be used for the variable,pattern=^[A-Z0-9_]+$"`
	Sensitive  bool                   `json:"sensitive,omitempty" jsonschema:"description=Whether to mark this variable as sensitive to not print it in the log"`
	AutoIndent bool                   `json:"autoIndent,omitempty" jsonschema:"description=Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_VAR_."`
	Pattern    string                 `json:"pattern,omitempty" jsonschema:"description=An optional regex pattern that a variable value must match before a package deployment can continue."`
	Type       variables.VariableType `json:"type,omitempty" jsonschema:"description=Changes the handling of a variable to load contents differently (i.e. from a file rather than as a raw variable - templated files should be kept below 1 MiB),enum=raw,enum=file"`
}

// ActionWait specifies a condition to wait for before continuing
type ActionWait struct {
	Cluster *ActionWaitCluster `json:"cluster,omitempty" jsonschema:"description=Wait for a condition to be met in the cluster before continuing. Only one of cluster or network can be specified."`
	Network *ActionWaitNetwork `json:"network,omitempty" jsonschema:"description=Wait for a condition to be met on the network before continuing. Only one of cluster or network can be specified."`
}

// ActionWaitCluster specifies a condition to wait for before continuing
type ActionWaitCluster struct {
	Kind       string `json:"kind" jsonschema:"description=The kind of resource to wait for,example=Pod,example=Deployment)"`
	Identifier string `json:"name" jsonschema:"description=The name of the resource or selector to wait for,example=podinfo,example=app&#61;podinfo"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"description=The namespace of the resource to wait for"`
	Condition  string `json:"condition,omitempty" jsonschema:"description=The condition or jsonpath state to wait for; defaults to exist, a special condition that will wait for the resource to exist,example=Ready,example=Available,'{.status.availableReplicas}'=23"`
}

// ActionWaitNetwork specifies a condition to wait for before continuing
type ActionWaitNetwork struct {
	Protocol string `json:"protocol" jsonschema:"description=The protocol to wait for,enum=tcp,enum=http,enum=https"`
	Address  string `json:"address" jsonschema:"description=The address to wait for,example=localhost:8080,example=1.1.1.1"`
	Code     int    `json:"code,omitempty" jsonschema:"description=The HTTP status code to wait for if using http or https,example=200,example=404"`
}

func (action Action) Validate() error {
	// Validate SetVariable
	for _, variable := range action.SetVariables {
		// Variable names must match only uppercase letters, numbers and underscores.
		// https://regex101.com/r/tfsEuZ/1
		if !regexp.MustCompile(`^[A-Z0-9_]+$`).MatchString(variable.Name) {
			return fmt.Errorf("setVariable name %q must be all uppercase and contain no special characters except _", variable.Name)
		}
	}

	if action.Wait != nil {
		// Validate only cmd or wait, not both
		if action.Cmd != "" {
			return fmt.Errorf("action %q cannot be both a command and wait action", action.Cmd)
		}

		// Validate only cluster or network, not both
		if action.Wait.Cluster != nil && action.Wait.Network != nil {
			return fmt.Errorf("a single wait action must contain only one of cluster or network")
		}

		// Validate at least one of cluster or network
		if action.Wait.Cluster == nil && action.Wait.Network == nil {
			return fmt.Errorf("a single wait action must contain only one of cluster or network")
		}
	}

	return nil
}
