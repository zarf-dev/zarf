// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1alpha1 holds the definition of the v1alpha1 Zarf Package
package v1alpha1

import (
	"github.com/invopop/jsonschema"
	"github.com/zarf-dev/zarf/src/api/v1alpha1/extensions"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

var (
	// Define allowed OS, an empty string means it is allowed on all operating systems
	// same as enums on ZarfComponentOnlyTarget
	supportedOS = []string{"linux", "darwin", "windows"}
)

// SupportedOS returns the supported operating systems.
//
// The supported operating systems are: linux, darwin, windows.
//
// An empty string signifies no OS restrictions.
func SupportedOS() []string {
	return supportedOS
}

// ZarfComponent is the primary functional grouping of assets to deploy by Zarf.
type ZarfComponent struct {
	// The name of the component.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`

	// Message to include during package deploy describing the purpose of this component.
	Description string `json:"description,omitempty"`

	// Determines the default Y/N state for installing this component on package deploy.
	Default bool `json:"default,omitempty"`

	// Do not prompt user to install this component.
	Required *bool `json:"required,omitempty"`

	// Filter when this component is included in package creation or deployment.
	Only ZarfComponentOnlyTarget `json:"only,omitempty"`

	// [Deprecated] Create a user selector field based on all components in the same group. This will be removed in Zarf v1.0.0. Consider using 'only.flavor' instead.
	DeprecatedGroup string `json:"group,omitempty" jsonschema:"deprecated=true"`

	// [Deprecated] Specify a path to a public key to validate signed online resources. This will be removed in Zarf v1.0.0.
	DeprecatedCosignKeyPath string `json:"cosignKeyPath,omitempty" jsonschema:"deprecated=true"`

	// Import a component from another Zarf package.
	Import ZarfComponentImport `json:"import,omitempty"`

	// Kubernetes manifests to be included in a generated Helm chart on package deploy.
	Manifests []ZarfManifest `json:"manifests,omitempty"`

	// Helm charts to install during package deploy.
	Charts []ZarfChart `json:"charts,omitempty"`

	// Datasets to inject into a container in the target cluster.
	DataInjections []ZarfDataInjection `json:"dataInjections,omitempty"`

	// Files or folders to place on disk during package deployment.
	Files []ZarfFile `json:"files,omitempty"`

	// List of OCI images to include in the package.
	Images []string `json:"images,omitempty"`

	// List of git repos to include in the package.
	Repos []string `json:"repos,omitempty"`

	// Extend component functionality with additional features.
	Extensions extensions.ZarfComponentExtensions `json:"extensions,omitempty"`

	// [Deprecated] (replaced by actions) Custom commands to run before or after package deployment. This will be removed in Zarf v1.0.0.
	DeprecatedScripts DeprecatedZarfComponentScripts `json:"scripts,omitempty" jsonschema:"deprecated=true"`

	// Custom commands to run at various stages of a package lifecycle.
	Actions ZarfComponentActions `json:"actions,omitempty"`
}

// RequiresCluster returns if the component requires a cluster connection to deploy.
func (c ZarfComponent) RequiresCluster() bool {
	hasImages := len(c.Images) > 0
	hasCharts := len(c.Charts) > 0
	hasManifests := len(c.Manifests) > 0
	hasRepos := len(c.Repos) > 0
	hasDataInjections := len(c.DataInjections) > 0

	if hasImages || hasCharts || hasManifests || hasRepos || hasDataInjections {
		return true
	}

	return false
}

// IsRequired returns if the component is required or not.
func (c ZarfComponent) IsRequired() bool {
	if c.Required != nil {
		return *c.Required
	}

	return false
}

// ZarfComponentOnlyTarget filters a component to only show it for a given local OS and cluster.
type ZarfComponentOnlyTarget struct {
	// Only deploy component to specified OS.
	LocalOS string `json:"localOS,omitempty" jsonschema:"enum=linux,enum=darwin,enum=windows"`
	// Only deploy component to specified clusters.
	Cluster ZarfComponentOnlyCluster `json:"cluster,omitempty"`
	// Only include this component when a matching '--flavor' is specified on 'zarf package create'.
	Flavor string `json:"flavor,omitempty"`
}

// ZarfComponentOnlyCluster represents the architecture and K8s cluster distribution to filter on.
type ZarfComponentOnlyCluster struct {
	// Only create and deploy to clusters of the given architecture.
	Architecture string `json:"architecture,omitempty" jsonschema:"enum=amd64,enum=arm64"`
	// A list of kubernetes distros this package works with (Reserved for future use).
	Distros []string `json:"distros,omitempty" jsonschema:"example=k3s,example=eks"`
}

// ZarfFile defines a file to deploy.
type ZarfFile struct {
	// Local folder or file path or remote URL to pull into the package.
	Source string `json:"source"`
	// (files only) Optional SHA256 checksum of the file.
	Shasum string `json:"shasum,omitempty"`
	// The absolute or relative path where the file or folder should be copied to during package deploy.
	Target string `json:"target"`
	// (files only) Determines if the file should be made executable during package deploy.
	Executable bool `json:"executable,omitempty"`
	// List of symlinks to create during package deploy.
	Symlinks []string `json:"symlinks,omitempty"`
	// Local folder or file to be extracted from a 'source' archive.
	ExtractPath string `json:"extractPath,omitempty"`
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	// The name of the chart within Zarf; note that this must be unique and does not need to be the same as the name in the chart repo.
	Name string `json:"name"`
	// The version of the chart to deploy; for git-based charts this is also the tag of the git repo by default (when not using the '@' syntax for 'repos').
	Version string `json:"version,omitempty"`
	// The URL of the OCI registry, chart repository, or git repo where the helm chart is stored.
	URL string `json:"url,omitempty" jsonschema:"example=OCI registry: oci://ghcr.io/stefanprodan/charts/podinfo,example=helm chart repo: https://stefanprodan.github.io/podinfo,example=git repo: https://github.com/stefanprodan/podinfo (note the '@' syntax for 'repos' is supported here too)"`
	// The name of a chart within a Helm repository (defaults to the Zarf name of the chart).
	RepoName string `json:"repoName,omitempty"`
	// (git repo only) The sub directory to the chart within a git repo.
	GitPath string `json:"gitPath,omitempty" jsonschema:"example=charts/your-chart"`
	// The path to a local chart's folder or .tgz archive.
	LocalPath string `json:"localPath,omitempty"`
	// The namespace to deploy the chart to.
	Namespace string `json:"namespace,omitempty"`
	// The name of the Helm release to create (defaults to the Zarf name of the chart).
	ReleaseName string `json:"releaseName,omitempty"`
	// Whether to not wait for chart resources to be ready before continuing.
	NoWait bool `json:"noWait,omitempty"`
	// List of local values file paths or remote URLs to include in the package; these will be merged together when deployed.
	ValuesFiles []string `json:"valuesFiles,omitempty"`
	// [alpha] List of variables to set in the Helm chart.
	Variables []ZarfChartVariable `json:"variables,omitempty"`
}

// ZarfChartVariable represents a variable that can be set for a Helm chart overrides.
type ZarfChartVariable struct {
	// The name of the variable.
	Name string `json:"name" jsonschema:"pattern=^[A-Z0-9_]+$"`
	// A brief description of what the variable controls.
	Description string `json:"description"`
	// The path within the Helm chart values where this variable applies.
	Path string `json:"path"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart.
type ZarfManifest struct {
	// A name to give this collection of manifests; this will become the name of the dynamically-created helm chart.
	Name string `json:"name"`
	// The namespace to deploy the manifests to.
	Namespace string `json:"namespace,omitempty"`
	// List of local K8s YAML files or remote URLs to deploy (in order).
	Files []string `json:"files,omitempty"`
	// Allow traversing directory above the current directory if needed for kustomization.
	KustomizeAllowAnyDirectory bool `json:"kustomizeAllowAnyDirectory,omitempty"`
	// List of local kustomization paths or remote URLs to include in the package.
	Kustomizations []string `json:"kustomizations,omitempty"`
	// Whether to not wait for manifest resources to be ready before continuing.
	NoWait bool `json:"noWait,omitempty"`
}

// DeprecatedZarfComponentScripts are scripts that run before or after a component is deployed.
type DeprecatedZarfComponentScripts struct {
	// Show the output of the script during package deployment.
	ShowOutput bool `json:"showOutput,omitempty"`
	// Timeout in seconds for the script.
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// Retry the script if it fails.
	Retry bool `json:"retry,omitempty"`
	// Scripts to run before the component is added during package create.
	Prepare []string `json:"prepare,omitempty"`
	// Scripts to run before the component is deployed.
	Before []string `json:"before,omitempty"`
	// Scripts to run after the component successfully deploys.
	After []string `json:"after,omitempty"`
}

// ZarfComponentActions are ActionSets that map to different zarf package operations.
type ZarfComponentActions struct {
	// Actions to run during package creation.
	OnCreate ZarfComponentActionSet `json:"onCreate,omitempty"`
	// Actions to run during package deployment.
	OnDeploy ZarfComponentActionSet `json:"onDeploy,omitempty"`
	// Actions to run during package removal.
	OnRemove ZarfComponentActionSet `json:"onRemove,omitempty"`
}

// ZarfComponentActionSet is a set of actions to run during a zarf package operation.
type ZarfComponentActionSet struct {
	// Default configuration for all actions in this set.
	Defaults ZarfComponentActionDefaults `json:"defaults,omitempty"`
	// Actions to run at the start of an operation.
	Before []ZarfComponentAction `json:"before,omitempty"`
	// Actions to run at the end of an operation.
	After []ZarfComponentAction `json:"after,omitempty"`
	// Actions to run if all operations succeed.
	OnSuccess []ZarfComponentAction `json:"onSuccess,omitempty"`
	// Actions to run if all operations fail.
	OnFailure []ZarfComponentAction `json:"onFailure,omitempty"`
}

// ZarfComponentActionDefaults sets the default configs for child actions.
type ZarfComponentActionDefaults struct {
	// Hide the output of commands during execution (default false).
	Mute bool `json:"mute,omitempty"`
	// Default timeout in seconds for commands (default to 0, no timeout).
	MaxTotalSeconds int `json:"maxTotalSeconds,omitempty"`
	// Retry commands given number of times if they fail (default 0).
	MaxRetries int `json:"maxRetries,omitempty"`
	// Working directory for commands (default CWD).
	Dir string `json:"dir,omitempty"`
	// Additional environment variables for commands.
	Env []string `json:"env,omitempty"`
	// (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems.
	Shell exec.Shell `json:"shell,omitempty"`
}

// ZarfComponentAction represents a single action to run during a zarf package operation.
type ZarfComponentAction struct {
	// Hide the output of the command during package deployment (default false).
	Mute *bool `json:"mute,omitempty"`
	// Timeout in seconds for the command (default to 0, no timeout for cmd actions and 300, 5 minutes for wait actions).
	MaxTotalSeconds *int `json:"maxTotalSeconds,omitempty"`
	// Retry the command if it fails up to given number of times (default 0).
	MaxRetries *int `json:"maxRetries,omitempty"`
	// The working directory to run the command in (default is CWD).
	Dir *string `json:"dir,omitempty"`
	// Additional environment variables to set for the command.
	Env []string `json:"env,omitempty"`
	// The command to run. Must specify either cmd or wait for the action to do anything.
	Cmd string `json:"cmd,omitempty"`
	// (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems.
	Shell *exec.Shell `json:"shell,omitempty"`
	// [Deprecated] (replaced by setVariables) (onDeploy/cmd only) The name of a variable to update with the output of the command. This variable will be available to all remaining actions and components in the package. This will be removed in Zarf v1.0.0.
	DeprecatedSetVariable string `json:"setVariable,omitempty"`
	// (onDeploy/cmd only) An array of variables to update with the output of the command. These variables will be available to all remaining actions and components in the package.
	SetVariables []variables.Variable `json:"setVariables,omitempty"`
	// Description of the action to be displayed during package execution instead of the command.
	Description string `json:"description,omitempty"`
	// Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info.
	Wait *ZarfComponentActionWait `json:"wait,omitempty"`
}

// JSONSchemaExtend extends the generated json schema during `zarf internal gen-config-schema`
func (ZarfComponentAction) JSONSchemaExtend(schema *jsonschema.Schema) {
	name, _ := schema.Properties.Get("setVariable")
	name.Pattern = variables.UppercaseNumberUnderscorePattern
}

// ZarfComponentActionWait specifies a condition to wait for before continuing
type ZarfComponentActionWait struct {
	// Wait for a condition to be met in the cluster before continuing. Only one of cluster or network can be specified.
	Cluster *ZarfComponentActionWaitCluster `json:"cluster,omitempty"`
	// Wait for a condition to be met on the network before continuing. Only one of cluster or network can be specified.
	Network *ZarfComponentActionWaitNetwork `json:"network,omitempty"`
}

// ZarfComponentActionWaitCluster specifies a condition to wait for before continuing
type ZarfComponentActionWaitCluster struct {
	// The kind of resource to wait for.
	Kind string `json:"kind" jsonschema:"example=Pod,example=Deployment"`
	// The name of the resource or selector to wait for.
	Name string `json:"name" jsonschema:"example=podinfo,example=app=podinfo"`
	// The namespace of the resource to wait for.
	Namespace string `json:"namespace,omitempty"`
	// The condition or jsonpath state to wait for; defaults to exist, a special condition that will wait for the resource to exist.
	Condition string `json:"condition,omitempty" jsonschema:"example=Ready,example=Available,'{.status.availableReplicas}'=23"`
}

// ZarfComponentActionWaitNetwork specifies a condition to wait for before continuing
type ZarfComponentActionWaitNetwork struct {
	// The protocol to wait for.
	Protocol string `json:"protocol" jsonschema:"enum=tcp,enum=http,enum=https"`
	// The address to wait for.
	Address string `json:"address" jsonschema:"example=localhost:8080,example=1.1.1.1"`
	// The HTTP status code to wait for if using http or https.
	Code int `json:"code,omitempty" jsonschema:"example=200,example=404"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	// The namespace to target for data injection.
	Namespace string `json:"namespace"`
	// The K8s selector to target for data injection.
	Selector string `json:"selector" jsonschema:"example=app=data-injection"`
	// The container name to target for data injection.
	Container string `json:"container"`
	// The path within the container to copy the data into.
	Path string `json:"path"`
}

// ZarfDataInjection is a data-injection definition.
type ZarfDataInjection struct {
	// Either a path to a local folder/file or a remote URL of a file to inject into the given target pod + container.
	Source string `json:"source"`
	// The target pod + container to inject the data into.
	Target ZarfContainerTarget `json:"target"`
	// Compress the data before transmitting using gzip. Note: this requires support for tar/gzip locally and in the target image.
	Compress bool `json:"compress,omitempty"`
}

// ZarfComponentImport structure for including imported Zarf components.
type ZarfComponentImport struct {
	// The name of the component to import from the referenced zarf.yaml.
	Name string `json:"name,omitempty"`
	// The path to the directory containing the zarf.yaml to import.
	Path string `json:"path,omitempty"`
	// [beta] The URL to a Zarf package to import via OCI.
	URL string `json:"url,omitempty" jsonschema:"pattern=^oci://.*$"`
}

// JSONSchemaExtend extends the generated json schema during `zarf internal gen-config-schema`
func (ZarfComponentImport) JSONSchemaExtend(schema *jsonschema.Schema) {
	path, _ := schema.Properties.Get("path")
	url, _ := schema.Properties.Get("url")

	notSchema := &jsonschema.Schema{
		Pattern: ZarfPackageTemplatePrefix,
	}

	path.Not = notSchema
	url.Not = notSchema
}
