// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

// Component is the primary functional grouping of assets to deploy by Zarf.
type Component struct {
	// The name of the component.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`
	// Message to include during package deploy describing the purpose of this component.
	Description string `json:"description,omitempty"`
	// Do not install this component unless explicitly requested. Defaults to false, meaning the component is required.
	Optional bool `json:"optional,omitempty"`
	ComponentSpec

	// Data injections removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	dataInjections []ZarfDataInjection
	// group removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	group string
}

// GetDeprecatedDataInjections returns the v1alpha1 data injections carried as a backwards-compatibility shim.
func (c Component) GetDeprecatedDataInjections() []ZarfDataInjection {
	return c.dataInjections
}

// GetDeprecatedGroup returns the v1alpha1 group carried as a backwards-compatibility shim.
func (c Component) GetDeprecatedGroup() string {
	return c.group
}

// GetImages returns all image names specified in the component, including those from ImageArchives.
func (c Component) GetImages() []string {
	images := []string{}
	for _, img := range c.Images {
		images = append(images, img.Name)
	}
	for _, ia := range c.ImageArchives {
		images = append(images, ia.Images...)
	}
	return images
}

// RequiresCluster returns true if the component requires a cluster connection to deploy.
func (c Component) RequiresCluster() bool {
	return len(c.Images) > 0 || len(c.Charts) > 0 || len(c.Manifests) > 0 || len(c.Repositories) > 0
}

// ComponentTarget filters a component to only apply for a given local OS, architecture, or flavor.
type ComponentTarget struct {
	// Only deploy component to specified OS.
	OS string `json:"os,omitempty" jsonschema:"enum=linux,enum=darwin,enum=windows"`
	// Only include component for the given package architecture.
	Architecture string `json:"architecture,omitempty" jsonschema:"enum=amd64,enum=arm64"`
	// Only include this component when a matching '--flavor' is specified on 'zarf package create'.
	Flavor string `json:"flavor,omitempty"`
}

// ComponentImport is a reference to imported Zarf component configs.
type ComponentImport struct {
	// Local file path references to component config files to import.
	Local []ComponentImportLocal `json:"local,omitempty"`
	// OCI URL references to remote component config files to import; pulled at create time.
	Remote []ComponentImportRemote `json:"remote,omitempty"`
}

// ComponentImportLocal is a local file path reference to a component config.
type ComponentImportLocal struct {
	// The local file path to the component config.
	Path string `json:"path"`
}

// ComponentImportRemote is a remote OCI URL reference to a component config.
type ComponentImportRemote struct {
	// The OCI URL of the remote component config.
	URL string `json:"url"`
}

// Service identifies which Zarf CLI service a component provides.
type Service string

const (
	// ServiceRegistry indicates a component that provides the in-cluster registry.
	ServiceRegistry Service = "registry"
	// ServiceSeedRegistry indicates a component that provides the seed registry used during init.
	ServiceSeedRegistry Service = "seed-registry"
	// ServiceInjector indicates a component that provides the Zarf injector.
	ServiceInjector Service = "injector"
	// ServiceAgent indicates a component that provides the Zarf agent.
	ServiceAgent Service = "agent"
	// ServiceGitServer indicates a component that provides the in-cluster git server.
	ServiceGitServer Service = "git-server"
)

// ServerSideApplyMode controls when server-side apply is used during deploy.
type ServerSideApplyMode string

const (
	// ServerSideApplyEnabled always uses server-side apply.
	ServerSideApplyEnabled ServerSideApplyMode = "true"
	// ServerSideApplyDisabled always uses client-side apply.
	ServerSideApplyDisabled ServerSideApplyMode = "false"
	// ServerSideApplyAuto uses server-side apply for fresh installs and matches the prior strategy on upgrade.
	ServerSideApplyAuto ServerSideApplyMode = "auto"
)

// KustomizeManifest defines kustomization settings for a manifest.
type KustomizeManifest struct {
	// List of local kustomization paths or remote URLs to include in the package.
	Files []string `json:"files,omitempty"`
	// Allow traversing directory above the current directory if needed for kustomization.
	AllowAnyDirectory bool `json:"allowAnyDirectory,omitempty"`
	// Enable kustomize plugins when building manifests.
	EnablePlugins bool `json:"enablePlugins,omitempty"`
}

// Manifest defines raw manifests Zarf will deploy as a helm chart.
type Manifest struct {
	// A name to give this collection of manifests; this will become the name of the dynamically-created helm chart.
	Name string `json:"name" jsonschema:"maxLength=40"`
	// The namespace to deploy the manifests to.
	Namespace string `json:"namespace,omitempty"`
	// List of local K8s YAML files or remote URLs to deploy (in order).
	Files []string `json:"files,omitempty"`
	// Kustomize settings for this manifest.
	Kustomize *KustomizeManifest `json:"kustomize,omitempty"`
	// Whether to skip waiting for manifest resources to be ready before continuing.
	SkipWait bool `json:"skipWait,omitempty"`
	// Controls whether Server-Side Apply (SSA) or client-side apply (CSA) is used during deploy. Defaults to "auto" when omitted.
	ServerSideApply ServerSideApplyMode `json:"serverSideApply,omitempty" jsonschema:"enum=true,enum=false,enum=auto"`
	// EnableValues enables go-template processing on these manifests during deploy.
	EnableValues bool `json:"enableValues,omitempty"`
}

// Chart defines a helm chart to be deployed.
type Chart struct {
	// The name of the chart within Zarf; note that this must be unique and does not need to be the same as the name in the chart repository.
	Name string `json:"name"`
	// The version of the chart. This field is removed from the schema, but kept as a backwards compatibility shim so v1alpha1 packages can be converted to v1beta1.
	version string
	// The Helm repository where the chart is stored.
	HelmRepository *HelmRepositorySource `json:"helmRepository,omitempty"`
	// The Git repository where the chart is stored.
	Git *GitSource `json:"git,omitempty"`
	// The local path where the chart is stored.
	Local *LocalSource `json:"local,omitempty"`
	// The OCI registry where the chart is stored.
	OCI *OCISource `json:"oci,omitempty"`
	// The namespace to deploy the chart to.
	Namespace string `json:"namespace,omitempty"`
	// The name of the Helm release to create (defaults to the Zarf name of the chart).
	ReleaseName string `json:"releaseName,omitempty"`
	// Whether to skip waiting for chart resources to be ready before continuing.
	SkipWait bool `json:"skipWait,omitempty"`
	// List of local values file paths or remote URLs to include in the package; these will be merged together when deployed.
	ValuesFiles []string `json:"valuesFiles,omitempty"`
	// List of value sources mapped to their Helm override targets.
	Values []ChartValue `json:"values,omitempty"`
	// Skip validation of the chart's values against its JSON schema. Defaults to false.
	SkipSchemaValidation bool `json:"skipSchemaValidation,omitempty"`
	// Controls whether Helm uses Server-Side Apply (SSA) or client-side apply (CSA) when deploying this chart. Defaults to "auto" when omitted.
	ServerSideApply ServerSideApplyMode `json:"serverSideApply,omitempty" jsonschema:"enum=true,enum=false,enum=auto"`

	// Chart variables removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	variables []ZarfChartVariable
}

// GetDeprecatedVersion returns the deprecated top-level chart version, used as a v1alpha1 backwards-compatibility shim.
func (c Chart) GetDeprecatedVersion() string {
	return c.version
}

// GetDeprecatedVariables returns the v1alpha1 chart variables carried as a backwards-compatibility shim.
func (c Chart) GetDeprecatedVariables() []ZarfChartVariable {
	return c.variables
}

// ChartValue maps a values source path to a Helm chart target path.
type ChartValue struct {
	// The source path for the value.
	SourcePath string `json:"sourcePath"`
	// The target path within the Helm chart values.
	TargetPath string `json:"targetPath"`
}

// HelmRepositorySource represents a Helm chart stored in a Helm repository.
type HelmRepositorySource struct {
	// The name of a chart within a Helm repository.
	Name string `json:"name,omitempty"`
	// The URL of the chart repository where the Helm chart is stored.
	URL string `json:"url"`
	// The version of the chart in the Helm repository.
	Version string `json:"version"`
}

// GitSource represents a Helm chart stored in a Git repository.
type GitSource struct {
	// The URL of the Git repository where the Helm chart is stored.
	URL string `json:"url"`
	// The subdirectory containing the chart within a Git repo.
	Path string `json:"path,omitempty"`
}

// LocalSource represents a Helm chart stored locally.
type LocalSource struct {
	// The path to a local chart's folder or .tgz archive.
	Path string `json:"path"`
}

// OCISource represents a Helm chart stored in an OCI registry.
type OCISource struct {
	// The URL of the OCI registry where the Helm chart is stored.
	URL string `json:"url"`
	// The version of the chart in the OCI registry.
	Version string `json:"version"`
}

// File defines a file to deploy.
type File struct {
	// Local folder or file path or remote URL to pull into the package.
	Source string `json:"source"`
	// Optional checksum of the file in the format <algorithm>:<checksum> (e.g. sha256:abc123). Defaults to sha256 if no algorithm is specified.
	Checksum string `json:"checksum,omitempty"`
	// The absolute or relative path where the file or folder should be copied to during package deploy.
	Destination string `json:"destination"`
	// Determines if the file should be made executable during package deploy.
	Executable bool `json:"executable,omitempty"`
	// List of symlinks to create during package deploy.
	Symlinks []string `json:"symlinks,omitempty"`
	// Local folder or file to be extracted from a 'source' archive.
	ExtractPath string `json:"extractPath,omitempty"`
	// EnableValues enables go-template processing on this file during deploy.
	EnableValues bool `json:"enableValues,omitempty"`
}

// Image defines an OCI image to include in the package.
type Image struct {
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

// ComponentActions are ActionSets that map to different Zarf package operations.
type ComponentActions struct {
	// Actions to run during package creation.
	OnCreate ComponentActionSet `json:"onCreate,omitempty"`
	// Actions to run during package deployment.
	OnDeploy ComponentActionSet `json:"onDeploy,omitempty"`
	// Actions to run during package removal.
	OnRemove ComponentActionSet `json:"onRemove,omitempty"`
}

// ComponentActionSet is a set of actions to run during a Zarf package operation.
type ComponentActionSet struct {
	// Default configuration for all actions in this set.
	Defaults ComponentActionDefaults `json:"defaults,omitempty"`
	// Actions to run at the start of an operation.
	Before []ComponentAction `json:"before,omitempty"`
	// Actions to run at the end of an operation if it succeeds.
	OnSuccess []ComponentAction `json:"onSuccess,omitempty"`
	// Actions to run if any operation in this set fails.
	OnFailure []ComponentAction `json:"onFailure,omitempty"`
}

// ComponentActionDefaults sets the default configs for child actions.
type ComponentActionDefaults struct {
	// Hide the output of commands during execution (default false).
	Silent bool `json:"silent,omitempty"`
	// Default timeout in seconds for commands (default 0, no timeout).
	MaxTotalSeconds int32 `json:"maxTotalSeconds,omitempty"`
	// Retry commands a given number of times if they fail (default 0).
	Retries int32 `json:"retries,omitempty"`
	// Working directory for commands (default CWD).
	Dir string `json:"dir,omitempty"`
	// Additional environment variables for commands.
	Env []string `json:"env,omitempty"`
	// Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems.
	Shell Shell `json:"shell,omitempty"`
}

// ComponentAction represents a single action to run during a Zarf package operation.
type ComponentAction struct {
	// Hide the output of the command during package deployment (default false).
	Silent *bool `json:"silent,omitempty"`
	// Timeout in seconds for the command (default 0, no timeout for cmd actions and 300, 5 minutes for wait actions).
	MaxTotalSeconds *int32 `json:"maxTotalSeconds,omitempty"`
	// Retry the command if it fails up to a given number of times (default 0).
	Retries *int32 `json:"retries,omitempty"`
	// The working directory to run the command in (default is CWD).
	Dir *string `json:"dir,omitempty"`
	// Additional environment variables to set for the command.
	Env []string `json:"env,omitempty"`
	// The command to run. Must specify either cmd or wait for the action to do anything.
	Cmd string `json:"cmd,omitempty"`
	// Indicates a preference for a shell for the provided cmd.
	Shell *Shell `json:"shell,omitempty"`
	// An array of values to set with the output of the command.
	SetValues []SetValue `json:"setValues,omitempty"`
	// Description of the action to be displayed during package execution instead of the command.
	Description string `json:"description,omitempty"`
	// Wait for a condition to be met before continuing.
	Wait *ComponentActionWait `json:"wait,omitempty"`
	// EnableValues enables go-template processing on the cmd field.
	EnableValues bool `json:"enableValues,omitempty"`
	// setVariables removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	setVariables []Variable
}

// GetDeprecatedSetVariables returns the v1alpha1 setVariables carried as a backwards-compatibility shim.
func (a ComponentAction) GetDeprecatedSetVariables() []Variable {
	return a.setVariables
}

// SetValueType declares the expected input back from the cmd, allowing structured data to be parsed.
type SetValueType string

const (
	// SetValueYAML enables YAML parsing.
	SetValueYAML SetValueType = "yaml"
	// SetValueJSON enables JSON parsing.
	SetValueJSON SetValueType = "json"
	// SetValueString sets the raw value.
	SetValueString SetValueType = "string"
)

// SetValue declares a value that can be set during a package deploy.
type SetValue struct {
	// Key represents which value to assign to.
	Key string `json:"key,omitempty"`
	// Type declares the kind of data being stored in the value.
	Type SetValueType `json:"type,omitempty"`
}

// ComponentActionWait specifies a condition to wait for before continuing.
type ComponentActionWait struct {
	// Wait for a condition to be met in the cluster before continuing. Only one of cluster or network can be specified.
	Cluster *ComponentActionWaitCluster `json:"cluster,omitempty"`
	// Wait for a condition to be met on the network before continuing. Only one of cluster or network can be specified.
	Network *ComponentActionWaitNetwork `json:"network,omitempty"`
}

// ComponentActionWaitCluster specifies a cluster-level condition to wait for.
type ComponentActionWaitCluster struct {
	// The kind of resource to wait for.
	Kind string `json:"kind" jsonschema:"example=Pod,example=Deployment"`
	// The name of the resource or selector to wait for.
	Name string `json:"name" jsonschema:"example=podinfo,example=app=podinfo"`
	// The namespace of the resource to wait for.
	Namespace string `json:"namespace,omitempty"`
	// The condition or jsonpath state to wait for; defaults to kstatus readiness checks.
	Condition string `json:"condition,omitempty" jsonschema:"example=Available,'{.status.availableReplicas}'=23"`
}

// ComponentActionWaitNetwork specifies a network-level condition to wait for.
type ComponentActionWaitNetwork struct {
	// The protocol to wait for.
	Protocol string `json:"protocol" jsonschema:"enum=tcp,enum=http,enum=https"`
	// The address to wait for.
	Address string `json:"address" jsonschema:"example=localhost:8080,example=1.1.1.1"`
	// The HTTP status code to wait for if using http or https.
	Code int32 `json:"code,omitempty" jsonschema:"example=200,example=404"`
}

// Shell represents the desired shell to use for a given command.
type Shell struct {
	// Windows shell preference.
	Windows string `json:"windows,omitempty" jsonschema:"example=powershell,example=cmd,example=pwsh"`
	// Linux shell preference.
	Linux string `json:"linux,omitempty" jsonschema:"example=sh,example=bash,example=zsh"`
	// Darwin (macOS) shell preference.
	Darwin string `json:"darwin,omitempty" jsonschema:"example=sh,example=bash,example=zsh"`
}
