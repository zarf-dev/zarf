// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package actions

import "time"

// ActionList contains a sequence of actions and their shared configuration.
type ActionList struct {
	Defaults Config
	Actions  []Action
}

// Config is the effective command configuration shared by actions in a list.
type Config struct {
	Silent  bool
	Timeout time.Duration
	Retries int
	Dir     string
	Env     []string
	Shell   Shell
}

// Action is a schema-neutral executable package action.
type Action struct {
	Silent         *bool
	Timeout        *time.Duration
	Retries        *int
	Dir            *string
	Env            []string
	Cmd            string
	Shell          *Shell
	SetValues      []ValueOutput
	SetVariables   []VariableOutput
	SetVariable    string
	Description    string
	Wait           *Wait
	ShouldTemplate bool
}

// ValueOutput stores command output in the deployment values map.
type ValueOutput struct {
	Key  string
	Type ValueOutputType
}

// ValueOutputType declares how a value output is decoded.
type ValueOutputType string

const (
	// ValueOutputYAML decodes command output as YAML.
	ValueOutputYAML ValueOutputType = "yaml"
	// ValueOutputJSON decodes command output as JSON.
	ValueOutputJSON ValueOutputType = "json"
	// ValueOutputString keeps command output as text.
	ValueOutputString ValueOutputType = "string"
)

// VariableOutput stores command output as a runtime variable.
type VariableOutput struct {
	Name       string
	Sensitive  bool
	AutoIndent bool
	Pattern    string
	Type       string
}

// Wait is an action that waits for a cluster or network condition.
type Wait struct {
	Cluster *ClusterWait
	Network *NetworkWait
}

// ClusterWait describes a Kubernetes resource condition.
type ClusterWait struct {
	Kind      string
	Name      string
	Namespace string
	Condition string
	// DefaultReady selects Kubernetes readiness when Condition is empty. Alpha
	// packages leave this false because their empty condition means "exists".
	DefaultReady bool
}

// NetworkWait describes a network condition.
type NetworkWait struct {
	Protocol string
	Address  string
	Code     int
}

// Shell is the preferred command shell for each supported OS.
type Shell struct {
	Windows string
	Linux   string
	Darwin  string
}
