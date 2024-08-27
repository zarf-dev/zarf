// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"github.com/invopop/jsonschema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZarfComponent is the primary functional grouping of assets to deploy by Zarf.
type ZarfComponent struct {
	// The name of the component.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`

	// Message to include during package deploy describing the purpose of this component.
	Description string `json:"description,omitempty"`

	// Determines the default Y/N state for installing this component on package deploy.
	Default bool `json:"default,omitempty"`

	// Do not prompt user to install this component. (Defaults to false)
	Optional *bool `json:"optional,omitempty"`

	// Filter when this component is included in package creation or deployment.
	Only ZarfComponentOnlyTarget `json:"only,omitempty"`

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

	// DeprecatedGroup will not be included in the schema and cannot be set by the users of v1beta1
	// This is here to maintain compatibility with v1alpha1 before the feature is removed in Zarf v1.0.0.
	DeprecatedGroup string `json:"group,omitempty" jsonschema:"-"`

	// DeprecatedExtensions will not be included in the schema and cannot be set by the users of v1beta1
	// This is here to maintain compatibility with v1alpha1 before the feature is removed in Zarf v1.0.0.
	DeprecatedExtensions ZarfComponentExtensions `json:"extensions,omitempty" jsonschema:"-"`

	// DeprecatedCosignKeyPath will not be included in the schema and cannot be set by the users of v1beta1
	// This is here to maintain compatibility with v1alpha1 before the feature is removed in Zarf v1.0.0.
	DeprecatedCosignKeyPath string `json:"cosignKeyPath,omitempty" jsonschema:"-"`

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

// IsOptional returns if the component is optional.
func (c ZarfComponent) IsOptional() bool {
	if c.Optional == nil {
		return false
	}
	return *c.Optional
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
	// The Helm repo where the chart is stored
	Helm HelmRepoSource `json:"helm,omitempty"`
	// The Git repo where the chart is stored
	Git GitRepoSource `json:"git,omitempty"`
	// The local path where the chart is stored
	Local LocalRepoSource `json:"local,omitempty"`
	// The OCI registry where the chart is stored
	OCI OCISource `json:"oci,omitempty"`
	// The version of the chart to deploy; for git-based charts this is also the tag of the git repo by default (when not using the '@' syntax for 'repos').
	Version string `json:"version,omitempty"`
	// The namespace to deploy the chart to.
	Namespace string `json:"namespace,omitempty"`
	// The name of the Helm release to create (defaults to the Zarf name of the chart).
	ReleaseName string `json:"releaseName,omitempty"`
	// Whether to not wait for chart resources to be ready before continuing.
	Wait *bool `json:"wait,omitempty"`
	// List of local values file paths or remote URLs to include in the package; these will be merged together when deployed.
	ValuesFiles []string `json:"valuesFiles,omitempty"`
	// [alpha] List of variables to set in the Helm chart.
	Variables []ZarfChartVariable `json:"variables,omitempty"`
}

// HelmRepoSource represents a Helm chart stored in a Helm repository.
type HelmRepoSource struct {
	// The name of a chart within a Helm repository (defaults to the Zarf name of the chart).
	RepoName string `json:"repoName,omitempty"`
	// The URL of the chart repository where the helm chart is stored.
	URL string `json:"url"`
}

// GitRepoSource represents a Helm chart stored in a Git repository.
type GitRepoSource struct {
	// The URL of the git repository where the helm chart is stored.
	URL string `json:"url"`
	// The sub directory to the chart within a git repo.
	Path string `json:"path,omitempty"`
	// The Tag of the repo where the helm chart is stored.
	Tag string `json:"tag,omitempty"`
}

// LocalRepoSource represents a Helm chart stored locally.
type LocalRepoSource struct {
	// The path to a local chart's folder or .tgz archive.
	Path string `json:"path,omitempty"`
}

// OCISource represents a Helm chart stored in an OCI registry.
type OCISource struct {
	// The URL of the OCI registry where the helm chart is stored.
	URL string `json:"url"`
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
	// Allow traversing directory above the current directory if needed for kustomization. (Defaults to false)
	KustomizeAllowAnyDirectory bool `json:"kustomizeAllowAnyDirectory,omitempty"`
	// List of local kustomization paths or remote URLs to include in the package.
	Kustomizations []string `json:"kustomizations,omitempty"`
	// Whether to not wait for manifest resources to be ready before continuing. (Defaults to true)
	Wait *bool `json:"wait,omitempty"`
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
	// Default timeout in seconds for commands (default no timeout).
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// Retry commands given number of times if they fail (default 0).
	Retries int `json:"retries,omitempty"`
	// Working directory for commands (default CWD).
	Dir string `json:"dir,omitempty"`
	// Additional environment variables for commands.
	Env []string `json:"env,omitempty"`
	// (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems.
	Shell Shell `json:"shell,omitempty"`
}

// ZarfComponentAction represents a single action to run during a zarf package operation.
type ZarfComponentAction struct {
	// Hide the output of the command during package deployment (default false).
	Mute *bool `json:"mute,omitempty"`
	// Timeout in seconds for the command (default to 0, no timeout for cmd actions and 5 minutes for wait actions).
	Timeout *metav1.Duration `json:"timeout,omitempty"`
	// Retry the command if it fails up to given number of times (default 0).
	Retries int `json:"retries,omitempty"`
	// The working directory to run the command in (default is CWD).
	Dir *string `json:"dir,omitempty"`
	// Additional environment variables to set for the command.
	Env []string `json:"env,omitempty"`
	// The command to run. Must specify either cmd or wait for the action to do anything.
	Cmd string `json:"cmd,omitempty"`
	// (cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems.
	Shell *Shell `json:"shell,omitempty"`
	// (onDeploy/cmd only) An array of variables to update with the output of the command. These variables will be available to all remaining actions and components in the package.
	SetVariables []Variable `json:"setVariables,omitempty"`
	// Description of the action to be displayed during package execution instead of the command.
	Description string `json:"description,omitempty"`
	// Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info.
	Wait *ZarfComponentActionWait `json:"wait,omitempty"`
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

// Shell represents the desired shell to use for a given command
type Shell struct {
	Windows string `json:"windows,omitempty" jsonschema:"description=(default 'powershell') Indicates a preference for the shell to use on Windows systems (note that choosing 'cmd' will turn off migrations like touch -> New-Item),example=powershell,example=cmd,example=pwsh,example=sh,example=bash,example=gsh"`
	Linux   string `json:"linux,omitempty" jsonschema:"description=(default 'sh') Indicates a preference for the shell to use on Linux systems,example=sh,example=bash,example=fish,example=zsh,example=pwsh"`
	Darwin  string `json:"darwin,omitempty" jsonschema:"description=(default 'sh') Indicates a preference for the shell to use on macOS systems,example=sh,example=bash,example=fish,example=zsh,example=pwsh"`
}

// ZarfComponentExtensions is a struct that contains all the official extensions.
type ZarfComponentExtensions struct {
	// Configurations for installing Big Bang and Flux in the cluster.
	BigBang *BigBang `json:"bigbang,omitempty" jsonschema:"-"`
}

// BigBang holds the configuration for the Big Bang extension.
type BigBang struct {
	// The version of Big Bang to use.
	Version string `json:"version" jsonschema:"-"`
	// Override repo to pull Big Bang from instead of Repo One.
	Repo string `json:"repo,omitempty" jsonschema:"-"`
	// The list of values files to pass to Big Bang; these will be merged together.
	ValuesFiles []string `json:"valuesFiles,omitempty" jsonschema:"-"`
	// Whether to skip deploying flux; Defaults to false.
	SkipFlux bool `json:"skipFlux,omitempty" jsonschema:"-"`
	// Optional paths to Flux kustomize strategic merge patch files.
	FluxPatchFiles []string `json:"fluxPatchFiles,omitempty" jsonschema:"-"`
}
