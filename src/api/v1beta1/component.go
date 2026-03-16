// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 holds the definition of the v1beta1 Zarf Package
package v1beta1

import (
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZarfComponent is the primary functional grouping of assets to deploy by Zarf.
type ZarfComponent struct {
	// The name of the component.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`

	// Message to include during package deploy describing the purpose of this component.
	Description string `json:"description,omitempty"`

	// Whether this component is default. Defaults to false.
	Default bool `json:"default,omitempty"`

	// Do not prompt user to install this component. Defaults to false, meaning the component is required.
	Optional *bool `json:"optional,omitempty"`

	// Filter when this component is included in package creation or deployment.
	Only ZarfComponentOnlyTarget `json:"only,omitempty"`

	// Import a component from another Zarf component config.
	Import ZarfComponentImport `json:"import,omitempty"`

	// Features of the Zarf CLI to enable for this component.
	Features ZarfComponentFeatures `json:"features,omitempty"`

	// Kubernetes manifests to be included in a generated Helm chart on package deploy.
	Manifests []ZarfManifest `json:"manifests,omitempty"`

	// Helm charts to install during package deploy.
	Charts []ZarfChart `json:"charts,omitempty"`

	// Files or folders to place on disk during package deployment.
	Files []ZarfFile `json:"files,omitempty"`

	// List of OCI images to include in the package.
	Images []ZarfImage `json:"images,omitempty"`

	// List of tar archives of images to include in the package.
	ImageArchives []ImageArchive `json:"imageArchives,omitempty"`

	// List of git repos to include in the package.
	Repos []string `json:"repos,omitempty"`

	// Custom commands to run at various stages of a package lifecycle.
	Actions ZarfComponentActions `json:"actions,omitempty"`

	// Datasets to inject into a container in the target cluster.
	// This field is not part of the v1beta1 schema but is kept as a backwards compatibility shim so v1alpha1 packages can be losslessly
	// converted to v1beta1 for packager logic.
	dataInjections []v1alpha1.ZarfDataInjection
}

// SetDataInjections allows setting data injections for lossless v1alpha1 conversions
func (c *ZarfComponent) SetDataInjections(dataInjections []v1alpha1.ZarfDataInjection) {
	c.dataInjections = dataInjections
}

// GetDataInjections is a shim to retrieving data injections when set for lossless v1alpha1 conversions
func (c ZarfComponent) GetDataInjections() []v1alpha1.ZarfDataInjection {
	return c.dataInjections
}

// RequiresCluster returns if the component requires a cluster connection to deploy.
func (c ZarfComponent) RequiresCluster() bool {
	hasImages := len(c.Images) > 0
	hasCharts := len(c.Charts) > 0
	hasManifests := len(c.Manifests) > 0
	hasRepos := len(c.Repos) > 0

	if hasImages || hasCharts || hasManifests || hasRepos {
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
	// Template enables go-template processing on this file during deploy.
	Template *bool `json:"template,omitempty"`
}

// ShouldTemplate returns whether go-template processing is enabled for this file.
func (f ZarfFile) ShouldTemplate() bool {
	if f.Template != nil {
		return *f.Template
	}
	return false
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	// The name of the chart within Zarf; note that this must be unique and does not need to be the same as the name in the chart repo.
	Name string `json:"name"`
	// The version of the chart. This field is not part of the v1beta1 schema but is kept
	// as a backwards compatibility shim so v1alpha1 packages can be converted to v1beta1.
	version string
	// The Helm repo where the chart is stored.
	HelmRepo HelmRepoSource `json:"helmRepo,omitempty"`
	// The Git repo where the chart is stored.
	Git GitRepoSource `json:"git,omitempty"`
	// The local path where the chart is stored.
	Local LocalRepoSource `json:"local,omitempty"`
	// The OCI registry where the chart is stored.
	OCI OCISource `json:"oci,omitempty"`
	// The namespace to deploy the chart to.
	Namespace string `json:"namespace,omitempty"`
	// The name of the Helm release to create (defaults to the Zarf name of the chart).
	ReleaseName string `json:"releaseName,omitempty"`
	// Whether to wait for chart resources to be ready before continuing. Defaults to true.
	Wait *bool `json:"wait,omitempty"`
	// List of local values file paths or remote URLs to include in the package; these will be merged together when deployed.
	ValuesFiles []string `json:"valuesFiles,omitempty"`
	// List of values sources to their Helm override target.
	Values []ZarfChartValue `json:"values,omitempty"`
	// Whether to validate the chart's values against its JSON schema. Defaults to true.
	SchemaValidation *bool `json:"schemaValidation,omitempty"`
	// Controls whether Helm uses Server-Side Apply (SSA) or client-side apply (CSA) when deploying this chart.
	//   - "true":  always use SSA
	//   - "false": always use CSA
	//   - "auto":  use SSA for fresh installs; for upgrades, match whichever strategy
	//              was used when the chart was first installed
	// Defaults to "auto" when omitted.
	ServerSideApply string `json:"serverSideApply,omitempty" jsonschema:"enum=true,enum=false,enum=auto"`
}

// ShouldRunSchemaValidation returns whether Helm schema validation should run.
func (zc ZarfChart) ShouldRunSchemaValidation() bool {
	if zc.SchemaValidation != nil {
		return *zc.SchemaValidation
	}
	return true
}

// GetServerSideApply returns server side apply with default of "auto" if it is not set
func (zc ZarfChart) GetServerSideApply() string {
	if zc.ServerSideApply == "" {
		return "auto"
	}
	return zc.ServerSideApply
}

// GetDeprecatedVersion gets the version of the chart, used as a backwards compatibility shim with v1alpha1.
func (zc ZarfChart) GetDeprecatedVersion() string {
	return zc.version
}

// SetDeprecatedVersion sets the version of the chart, used as a backwards compatibility shim with v1alpha1.
// This function will be deleted when v1alpha1 packages are no longer deployable
func (zc *ZarfChart) SetDeprecatedVersion(version string) {
	zc.version = version
}

// ZarfChartValue maps a values source path to a Helm chart target path.
type ZarfChartValue struct {
	// The source path for the value.
	SourcePath string `json:"sourcePath"`
	// The target path within the Helm chart values.
	TargetPath string `json:"targetPath"`
}

// HelmRepoSource represents a Helm chart stored in a Helm repository.
type HelmRepoSource struct {
	// The name of a chart within a Helm repository (defaults to the Zarf name of the chart).
	Name string `json:"name,omitempty"`
	// The URL of the chart repository where the helm chart is stored.
	URL string `json:"url"`
	// The version of the chart to deploy.
	Version string `json:"version"`
}

// GitRepoSource represents a Helm chart stored in a Git repository.
type GitRepoSource struct {
	// The URL of the git repository where the helm chart is stored.
	URL string `json:"url"`
	// The sub directory to the chart within a git repo.
	Path string `json:"path,omitempty"`
}

// LocalRepoSource represents a Helm chart stored locally.
type LocalRepoSource struct {
	// The path to a local chart's folder or .tgz archive.
	Path string `json:"path"`
}

// OCISource represents a Helm chart stored in an OCI registry.
type OCISource struct {
	// The URL of the OCI registry where the helm chart is stored.
	URL string `json:"url"`
	// The version of the chart to deploy.
	Version string `json:"version"`
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
	// Controls whether Server-Side Apply (SSA) or client-side apply (CSA) is used during deploy.
	//   - "true":  always use SSA
	//   - "false": always use CSA
	//   - "auto":  use SSA for fresh installs; for upgrades, match whichever strategy
	//              was used when the chart was first installed
	// Defaults to "auto" when omitted.
	ServerSideApply string `json:"serverSideApply,omitempty" jsonschema:"enum=true,enum=false,enum=auto"`
	// Whether to wait for manifest resources to be ready before continuing. Defaults to true.
	Wait *bool `json:"wait,omitempty"`
	// Template enables go-template processing on these manifests during deploy.
	Template *bool `json:"template,omitempty"`
}

// ShouldTemplate returns whether go-template processing is enabled for this manifest.
func (m ZarfManifest) ShouldTemplate() bool {
	if m.Template != nil {
		return *m.Template
	}
	return false
}

// GetServerSideApply returns server side apply with default of "auto" if it is not set
func (m ZarfManifest) GetServerSideApply() string {
	if m.ServerSideApply == "" {
		return "auto"
	}
	return m.ServerSideApply
}

// ZarfImage defines an OCI image to include in the package.
type ZarfImage struct {
	// The image reference.
	Name string `json:"name"`
	// The source to pull the image from. Defaults to "registry".
	Source string `json:"source,omitempty" jsonschema:"enum=registry,enum=daemon"`
}

// ImageArchive defines a tar archive of images to include in the package.
type ImageArchive struct {
	// The path to the tar archive.
	Path string `json:"path"`
	// The list of images contained in the archive.
	Images []string `json:"images"`
}

// ZarfComponentFeatures defines features of the Zarf CLI to enable for a component.
type ZarfComponentFeatures struct {
	// Whether this component provides a registry.
	IsRegistry bool `json:"isRegistry,omitempty"`
	// Injector configuration for the component.
	Injector *Injector `json:"injector,omitempty"`
	// Whether this component provides an agent.
	IsAgent bool `json:"isAgent,omitempty"`
}

// Injector defines the configuration for the Zarf injector.
type Injector struct {
	// Whether the injector is enabled.
	Enabled bool `json:"enabled"`
	// Values for the injector.
	Values *InjectorValues `json:"values,omitempty"`
}

// InjectorValues defines configurable values for the Zarf injector.
type InjectorValues struct {
	// Tolerations for the injector pod.
	Tolerations string `json:"tolerations,omitempty"`
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
	// Actions to run if all operations fail.
	OnFailure []ZarfComponentAction `json:"onFailure,omitempty"`
}

// ZarfComponentActionDefaults sets the default configs for child actions.
type ZarfComponentActionDefaults struct {
	// Hide the output of commands during execution (default false).
	Mute bool `json:"mute,omitempty"`
	// Default timeout for commands (default no timeout).
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
	// Timeout for the command (default to 0, no timeout for cmd actions and 5 minutes for wait actions).
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
	// An array of values to set with the output of the command.
	SetValues []SetValue `json:"setValues,omitempty"`
	// Description of the action to be displayed during package execution instead of the command.
	Description string `json:"description,omitempty"`
	// Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info.
	Wait *ZarfComponentActionWait `json:"wait,omitempty"`
	// Disable go-template processing on the cmd field.
	Template *bool `json:"template,omitempty"`
}

// ShouldTemplate returns whether the action should have go-template processing.
func (a ZarfComponentAction) ShouldTemplate() bool {
	if a.Template != nil {
		return *a.Template
	}
	return false
}

// SetValueType declares the expected input back from the cmd, allowing structured data to be parsed.
type SetValueType string

// SetValueYAML enables YAML parsing.
var SetValueYAML = SetValueType("yaml")

// SetValueJSON enables JSON parsing.
var SetValueJSON = SetValueType("json")

// SetValueString sets the raw value.
var SetValueString = SetValueType("string")

// SetValue declares a value that can be set during a package deploy.
type SetValue struct {
	// Key represents which value to assign to.
	Key string `json:"key,omitempty"`
	// Value is the current value at the key.
	Value any `json:"value,omitempty"`
	// Type declares the kind of data being stored in the value. JSON and YAML types ensure proper formatting when
	// inserting the value into the template. Defaults to SetValueString behavior when empty.
	Type SetValueType `json:"type,omitempty"`
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
	// The condition or jsonpath state to wait for; defaults to kstatus readiness checks.
	Condition string `json:"condition,omitempty" jsonschema:"example=Available,'{.status.availableReplicas}'=23"`
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

// ZarfComponentImport structure for including imported Zarf components.
type ZarfComponentImport struct {
	// The path to the component config file to import.
	Path string `json:"path,omitempty"`
	// The URL to a Zarf component config to import via OCI.
	URL string `json:"url,omitempty" jsonschema:"pattern=^oci://.*$"`
}

// Shell represents the desired shell to use for a given command
type Shell struct {
	Windows string `json:"windows,omitempty" jsonschema:"description=(default 'powershell') Indicates a preference for the shell to use on Windows systems (note that choosing 'cmd' will turn off migrations like touch -> New-Item),example=powershell,example=cmd,example=pwsh,example=sh,example=bash,example=gsh"`
	Linux   string `json:"linux,omitempty" jsonschema:"description=(default 'sh') Indicates a preference for the shell to use on Linux systems,example=sh,example=bash,example=fish,example=zsh,example=pwsh"`
	Darwin  string `json:"darwin,omitempty" jsonschema:"description=(default 'sh') Indicates a preference for the shell to use on macOS systems,example=sh,example=bash,example=fish,example=zsh,example=pwsh"`
}
