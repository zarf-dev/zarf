// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api

// ComponentActions are the actions associated with each package lifecycle operation.
type ComponentActions struct {
	OnCreate ActionSet
	OnDeploy ActionSet
	OnRemove ActionSet
}

// ActionSet contains actions for one package lifecycle operation.
// FIXME: actions should be folded back into the type
type ActionSet struct {
	Defaults ActionDefaults
	// DefaultsDefined preserves whether a source schema explicitly supplied defaults.
	// FIXME: this must be deleted
	DefaultsDefined bool
	Before          []Action
	After           []Action
	OnSuccess       []Action
	OnFailure       []Action
}

// ActionDefaults configures every action in an ActionSet unless the action overrides it.
type ActionDefaults struct {
	Silent          bool
	MaxTotalSeconds int
	Retries         int
	Dir             string
	Env             []string
	Shell           Shell
}

// Action is a command or wait operation performed during a package lifecycle operation.
type Action struct {
	Silent           *bool
	MaxTotalSeconds  *int
	Retries          *int
	Dir              *string
	Env              []string
	Cmd              string
	Shell            *Shell
	SetVariables     []ActionVariable
	SetValues        []SetValue
	Description      string
	Wait             *ActionWait
	EnableTemplating bool
	// Template preserves v1alpha1's explicit templating setting for lossless conversion.
	Template *bool
	// DeprecatedSetVariable preserves the deprecated v1alpha1 action field during conversion.
	DeprecatedSetVariable string
}

// ActionVariable receives an action's command output.
type ActionVariable struct {
	Name       string
	Sensitive  bool
	AutoIndent bool
	Pattern    string
	Type       string
}

// SetValue declares how command output is stored in the package values map.
type SetValue struct {
	Key   string
	Value any
	Type  SetValueType
}

// SetValueType declares the expected output format of an action command.
type SetValueType string

const (
	// SetValueYAML parses command output as YAML.
	SetValueYAML SetValueType = "yaml"
	// SetValueJSON parses command output as JSON.
	SetValueJSON SetValueType = "json"
	// SetValueString stores command output without parsing.
	SetValueString SetValueType = "string"
)

// Shell identifies the preferred command shell on each supported operating system.
type Shell struct {
	Windows string
	Linux   string
	Darwin  string
}

// ActionWait specifies a cluster or network condition to wait for.
type ActionWait struct {
	Cluster *ActionWaitCluster
	Network *ActionWaitNetwork
}

// ActionWaitCluster specifies a cluster-level condition to wait for.
type ActionWaitCluster struct {
	Kind      string
	Name      string
	Namespace string
	Condition WaitCondition
}

// WaitCondition carries both an explicit condition and the semantic default used when it is empty.
type WaitCondition struct {
	Expression string
	Default    WaitDefault
}

// WaitDefault is the behavior used when a cluster wait condition is omitted.
type WaitDefault string

const (
	// WaitForExistence waits for a resource to exist when no condition is supplied.
	WaitForExistence WaitDefault = "existence"
	// WaitForReadiness waits for a resource to be reconciled when no condition is supplied.
	WaitForReadiness WaitDefault = "readiness"
)

// ActionWaitNetwork specifies a network-level condition to wait for.
type ActionWaitNetwork struct {
	Protocol string
	Address  string
	Code     int
}
