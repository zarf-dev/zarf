// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import "github.com/defenseunicorns/zarf/src/types/extensions"

// ZarfComponent is the primary functional grouping of assets to deploy by Zarf.
type ZarfComponent struct {
	// The name of the component, must be unique to the package
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9\\-]+$"`

	// Description to include during package deploy describing the purpose of this component
	Description string `json:"description,omitempty"`

	// Default changes the default option when deploying this component
	// and determines the default Y/N state for installing this component
	// on package deploy
	Default bool `json:"default,omitempty"`

	// Required makes this component mandatory for package deployment
	// Do not prompt user to install this component, always install on package deploy"`
	Required bool `json:"required,omitempty"`

	// Only include compatible components during package deployment
	// Filter when this component is included in package creation or deployment"`
	Only ZarfComponentOnlyTarget `json:"only,omitempty"`

	// Key to match other components to produce a user selector field, used to create a BOOLEAN XOR for a set of components
	// Note: ignores default and required flags
	// Create a user selector field based on all components in the same group"`
	Group string `json:"group,omitempty"`

	//Path to cosign publickey for signed online resources
	// Specify a path to a public key to validate signed online resources"`
	CosignKeyPath string `json:"cosignKeyPath,omitempty"`

	// Import refers to another zarf.yaml package component.
	// Import a component from another Zarf package"`
	Import ZarfComponentImport `json:"import,omitempty"`

	// (Deprecated) DeprecatedScripts are custom commands that run before or after package deployment
	// [Deprecated] (replaced by actions) Custom commands to run before or after package deployment,deprecated=true"`
	DeprecatedScripts DeprecatedZarfComponentScripts `json:"scripts,omitempty"`

	// Replaces scripts, fine-grained control over commands to run at various stages of a package lifecycle
	// Custom commands to run at various stages of a package lifecycle"`
	Actions ZarfComponentActions `json:"actions,omitempty"`

	// Files are files to place on disk during deploy
	// Files to place on disk during package deployment"`
	Files []ZarfFile `json:"files,omitempty"`

	// Charts are helm charts to install during package deploy
	// Helm charts to install during package deploy"`
	Charts []ZarfChart `json:"charts,omitempty"`

	// Manifests are raw manifests that get converted into zarf-generated helm charts during deploy
	// Kubernetes manifests to be included in a generated Helm chart on package deploy"`
	Manifests []ZarfManifest `json:"manifests,omitempty"`

	// Images are the online images needed to be included in the zarf package
	// List of OCI images to include in the package"`
	Images []string `json:"images,omitempty"`

	// Collection of git repos to include in the package
	Repos []string `json:"repos,omitempty"`

	// Collection of data i to inject into a container in the target cluster
	DataInjections []ZarfDataInjection `json:"dataInjections,omitempty"`

	// Extend component functionality with additional features
	Extensions extensions.ZarfComponentExtensions `json:"extensions,omitempty"`
}

// ZarfComponentOnlyTarget filters a component to only show it for a given local OS and cluster.
type ZarfComponentOnlyTarget struct {
	// Only deploy component to specified OS,enum=linux,enum=darwin,enum=windows"`
	LocalOS string `json:"localOS,omitempty"`
	// Only deploy component to specified clusters"`
	Cluster ZarfComponentOnlyCluster `json:"cluster,omitempty"`
}

// ZarfComponentOnlyCluster represents the architecture and K8s cluster distribution to filter on.
type ZarfComponentOnlyCluster struct {
	// Only create and deploy to clusters of the given architecture,enum=amd64,enum=arm64"`
	Architecture string `json:"architecture,omitempty"`
	// A list of kubernetes distros this package works with (Reserved for future use),example=k3s,example=eks"`
	Distros []string `json:"distros,omitempty"`
}

// ZarfFile defines a file to deploy.
type ZarfFile struct {
	// Local file path or remote URL to pull into the package"`
	Source string `json:"source"`
	// Optional SHA256 checksum of the file"`
	Shasum string `json:"shasum,omitempty"`
	// The absolute or relative path where the file should be copied to during package deploy"`
	Target string `json:"target"`
	// Determines if the file should be made executable during package deploy"`
	Executable bool `json:"executable,omitempty"`
	// List of symlinks to create during package deploy"`
	Symlinks []string `json:"symlinks,omitempty"`
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	// The name of the chart to deploy; this should be the name of the chart as it is installed in the helm repo"`
	Name string `json:"name"`
	// The name of the release to create; defaults to the name of the chart"`
	ReleaseName string `json:"releaseName,omitempty"`
	URL         string `json:"url,omitempty" jsonschema:"oneof_required=url,example=OCI registry: oci://ghcr.io/stefanprodan/charts/podinfo,example=helm chart repo: https://stefanprodan.github.io/podinfo,example=git repo: https://github.com/stefanprodan/podinfo" jsonschema_description:"The URL of the OCI registry, chart repository, or git repo where the helm chart is stored"`
	// The version of the chart to deploy; for git-based charts this is also the tag of the git repo"`
	Version string `json:"version"`
	// The namespace to deploy the chart to"`
	Namespace string `json:"namespace"`
	// List of values files to include in the package; these will be merged together
	ValuesFiles []string `json:"valuesFiles,omitempty"`
	// The path to the chart in the repo if using a git repo instead of a helm repo,example=charts/your-chart"`
	GitPath   string `json:"gitPath,omitempty"`
	LocalPath string `json:"localPath,omitempty" jsonschema:"oneof_required=localPath,description=The path to the chart folder"`
	// Whether to not wait for chart resources to be ready before continuing"`
	NoWait bool `json:"noWait,omitempty"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart.
type ZarfManifest struct {
	// A name to give this collection of manifests; this will become the name of the dynamically-created helm chart"`
	Name string `json:"name"`
	// The namespace to deploy the manifests to"`
	Namespace string `json:"namespace,omitempty"`
	// List of individual K8s YAML files to deploy (in order)"`
	Files []string `json:"files,omitempty"`
	// Allow traversing directory above the current directory if needed for kustomization"`
	KustomizeAllowAnyDirectory bool `json:"kustomizeAllowAnyDirectory,omitempty"`
	// List of kustomization paths to include in the package"`
	Kustomizations []string `json:"kustomizations,omitempty"`
	// Whether to not wait for manifest resources to be ready before continuing"`
	NoWait bool `json:"noWait,omitempty"`
}

// DeprecatedZarfComponentScripts are scripts that run before or after a component is deployed
type DeprecatedZarfComponentScripts struct {
	// Show the output of the script during package deployment"`
	ShowOutput bool `json:"showOutput,omitempty"`
	// Timeout in seconds for the script"`
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// Retry the script if it fails"`
	Retry bool `json:"retry,omitempty"`
	// Scripts to run before the component is added during package create"`
	Prepare []string `json:"prepare,omitempty"`
	// Scripts to run before the component is deployed"`
	Before []string `json:"before,omitempty"`
	// Scripts to run after the component successfully deploys"`
	After []string `json:"after,omitempty"`
}

// ZarfComponentActions are actionsets that map to different zarf package operations
type ZarfComponentActions struct {
	// Actions to run during package creation"`
	OnCreate ZarfComponentActionSet `json:"onCreate,omitempty"`
	// Actions to run during package deployment"`
	OnDeploy ZarfComponentActionSet `json:"onDeploy,omitempty"`
	// Actions to run during package removal"`
	OnRemove ZarfComponentActionSet `json:"onRemove,omitempty"`
}

// ZarfComponentActionSet is a set of actions to run during a zarf package operation
type ZarfComponentActionSet struct {
	// Default configuration for all actions in this set"`
	Defaults ZarfComponentActionDefaults `json:"defaults,omitempty"`
	// Actions to run at the start of an operation"`
	Before []ZarfComponentAction `json:"before,omitempty"`
	// Actions to run at the end of an operation"`
	After []ZarfComponentAction `json:"after,omitempty"`
	// Actions to run if all operations succeed"`
	OnSuccess []ZarfComponentAction `json:"onSuccess,omitempty"`
	// Actions to run if all operations fail"`
	OnFailure []ZarfComponentAction `json:"onFailure,omitempty"`
}

// ZarfComponentActionDefaults sets the default configs for child actions
type ZarfComponentActionDefaults struct {
	// Hide the output of commands during execution (default false)"`
	Mute bool `json:"mute,omitempty"`
	// Default timeout in seconds for commands (default to 0, no timeout)"`
	MaxTotalSeconds int `json:"maxTotalSeconds,omitempty"`
	// Retry commands given number of times if they fail (default 0)"`
	MaxRetries int `json:"maxRetries,omitempty"`
	// Working directory for commands (default CWD)"`
	Dir string `json:"dir,omitempty"`
	// Additional environment variables for commands"`
	Env []string `json:"env,omitempty"`
	// (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems"`
	Shell ZarfComponentActionShell `json:"shell,omitempty"`
}

// ZarfComponentActionShell represents the desired shell to use for a given command
type ZarfComponentActionShell struct {
	// (default 'powershell') Indicates a preference for the shell to use on Windows systems (note that choosing 'cmd' will turn off migrations like touch -> New-Item),example=powershell,example=cmd,example=pwsh,example=sh,example=bash,example=gsh"`
	Windows string `json:"windows,omitempty"`
	// (default 'sh') Indicates a preference for the shell to use on Linux systems,example=sh,example=bash,example=fish,example=zsh,example=pwsh"`
	Linux string `json:"linux,omitempty"`
	// (default 'sh') Indicates a preference for the shell to use on macOS systems,example=sh,example=bash,example=fish,example=zsh,example=pwsh"`
	Darwin string `json:"darwin,omitempty"`
}

// ZarfComponentAction represents a single action to run during a zarf package operation
type ZarfComponentAction struct {
	// Hide the output of the command during package deployment (default false)"`
	Mute *bool `json:"mute,omitempty"`
	// Timeout in seconds for the command (default to 0, no timeout for cmd actions and 300, 5 minutes for wait actions)"`
	MaxTotalSeconds *int `json:"maxTotalSeconds,omitempty"`
	// Retry the command if it fails up to given number of times (default 0)"`
	MaxRetries *int `json:"maxRetries,omitempty"`
	// The working directory to run the command in (default is CWD)"`
	Dir *string `json:"dir,omitempty"`
	// Additional environment variables to set for the command"`
	Env []string `json:"env,omitempty"`
	// The command to run. Must specify either cmd or wait for the action to do anything."`
	Cmd string `json:"cmd,omitempty"`
	// (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems"`
	Shell *ZarfComponentActionShell `json:"shell,omitempty"`
	// [Deprecated] (replaced by setVariables) (onDeploy/cmd only) The name of a variable to update with the output of the command. This variable will be available to all remaining actions and components in the package.,pattern=^[A-Z0-9_]+$"`
	DeprecatedSetVariable string `json:"setVariable,omitempty"`
	// (onDeploy/cmd only) An array of variables to update with the output of the command. These variables will be available to all remaining actions and components in the package."`
	SetVariables []ZarfComponentActionSetVariable `json:"setVariables,omitempty"`
	// Description of the action to be displayed during package execution instead of the command"`
	Description string `json:"description,omitempty"`
	// Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info."`
	Wait *ZarfComponentActionWait `json:"wait,omitempty"`
}

// ZarfComponentActionSetVariable represents a variable that is to be set via an action
type ZarfComponentActionSetVariable struct {
	// The name to be used for the variable,pattern=^[A-Z0-9_]+$"`
	Name string `json:"name"`
	// Whether to mark this variable as sensitive to not print it in the Zarf log"`
	Sensitive bool `json:"sensitive,omitempty"`
	// Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_VAR_."`
	AutoIndent bool `json:"autoIndent,omitempty"`
}

// ZarfComponentActionWait specifies a condition to wait for before continuing
type ZarfComponentActionWait struct {
	// Wait for a condition to be met in the cluster before continuing. Only one of cluster or network can be specified."`
	Cluster *ZarfComponentActionWaitCluster `json:"cluster,omitempty"`
	// Wait for a condition to be met on the network before continuing. Only one of cluster or network can be specified."`
	Network *ZarfComponentActionWaitNetwork `json:"network,omitempty"`
}

// ZarfComponentActionWaitCluster specifies a condition to wait for before continuing
type ZarfComponentActionWaitCluster struct {
	// The kind of resource to wait for,example=Pod,example=Deployment)"`
	Kind string `json:"kind"`
	// The name of the resource or selector to wait for,example=podinfo,example=app&#61;podinfo"`
	Identifier string `json:"name"`
	// The namespace of the resource to wait for"`
	Namespace string `json:"namespace,omitempty"`
	// The condition to wait for; defaults to exist, a special condition that will wait for the resource to exist,example=Ready,example=Available"`
	Condition string `json:"condition,omitempty"`
}

// ZarfComponentActionWaitNetwork specifies a condition to wait for before continuing
type ZarfComponentActionWaitNetwork struct {
	// The protocol to wait for,enum=tcp,enum=http,enum=https"`
	Protocol string `json:"protocol"`
	// The address to wait for,example=localhost:8080,example=1.1.1.1"`
	Address string `json:"address"`
	// The HTTP status code to wait for if using http or https,example=200,example=404"`
	Code int `json:"code,omitempty"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	// The namespace to target for data injection"`
	Namespace string `json:"namespace"`
	// The K8s selector to target for data injection,example=app&#61;data-injection"`
	Selector string `json:"selector"`
	// The container name to target for data injection"`
	Container string `json:"container"`
	// The path within the container to copy the data into"`
	Path string `json:"path"`
}

// ZarfDataInjection is a data-injection definition.
type ZarfDataInjection struct {
	// A path to a local folder or file to inject into the given target pod + container"`
	Source string `json:"source"`
	// The target pod + container to inject the data into"`
	Target ZarfContainerTarget `json:"target"`
	// Compress the data before transmitting using gzip.  Note: this requires support for tar/gzip locally and in the target image."`
	Compress bool `json:"compress,omitempty"`
}

// ZarfComponentImport structure for including imported Zarf components.
type ZarfComponentImport struct {
	// The name of the component to import from the referenced zarf.yaml"`
	ComponentName string `json:"name,omitempty"`
	// For further explanation see https://regex101.com/library/Ldx8yG and https://regex101.com/r/Ldx8yG/1
	// The relative path to a directory containing a zarf.yaml to import from,pattern=^(?!.*###ZARF_PKG_TMPL_).*$"`
	Path string `json:"path"`
}
