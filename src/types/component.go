// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"slices"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types/extensions"
)

// ZarfComponent is the primary functional grouping of assets to deploy by Zarf.
type ZarfComponent struct {
	// Name is the unique identifier for this component
	Name string `json:"name" jsonschema:"description=The name of the component,pattern=^[a-z0-9\\-]*[a-z0-9]$"`

	// Description is a message given to a user when deciding to enable this component or not
	Description string `json:"description,omitempty" jsonschema:"description=Message to include during package deploy describing the purpose of this component"`

	// Default changes the default option when deploying this component
	Default bool `json:"default,omitempty" jsonschema:"description=Determines the default Y/N state for installing this component on package deploy"`

	// Required makes this component mandatory for package deployment
	Required *bool `json:"required,omitempty" jsonschema:"description=Do not prompt user to install this component, always install on package deploy."`

	// Only include compatible components during package deployment
	Only ZarfComponentOnlyTarget `json:"only,omitempty" jsonschema:"description=Filter when this component is included in package creation or deployment"`

	// DeprecatedGroup is a key to match other components to produce a user selector field, used to create a BOOLEAN XOR for a set of components
	//
	// Note: ignores default and required flags
	DeprecatedGroup string `json:"group,omitempty" jsonschema:"description=[Deprecated] Create a user selector field based on all components in the same group. This will be removed in Zarf v1.0.0. Consider using 'only.flavor' instead.,deprecated=true"`

	// DeprecatedCosignKeyPath to cosign public key for signed online resources
	DeprecatedCosignKeyPath string `json:"cosignKeyPath,omitempty" jsonschema:"description=[Deprecated] Specify a path to a public key to validate signed online resources. This will be removed in Zarf v1.0.0.,deprecated=true"`

	// Import refers to another zarf.yaml package component.
	Import ZarfComponentImport `json:"import,omitempty" jsonschema:"description=Import a component from another Zarf package"`

	// Manifests are raw manifests that get converted into zarf-generated helm charts during deploy
	Manifests []ZarfManifest `json:"manifests,omitempty" jsonschema:"description=Kubernetes manifests to be included in a generated Helm chart on package deploy"`

	// Charts are helm charts to install during package deploy
	Charts []ZarfChart `json:"charts,omitempty" jsonschema:"description=Helm charts to install during package deploy"`

	// Data packages to push into a running cluster
	DataInjections []ZarfDataInjection `json:"dataInjections,omitempty" jsonschema:"description=Datasets to inject into a container in the target cluster"`

	// Files are files to place on disk during deploy
	Files []ZarfFile `json:"files,omitempty" jsonschema:"description=Files or folders to place on disk during package deployment"`

	// Images are the online images needed to be included in the zarf package
	Images []string `json:"images,omitempty" jsonschema:"description=List of OCI images to include in the package"`

	// Repos are any git repos that need to be pushed into the git server
	Repos []string `json:"repos,omitempty" jsonschema:"description=List of git repos to include in the package"`

	// Extensions provide additional functionality to a component
	Extensions extensions.ZarfComponentExtensions `json:"extensions,omitempty" jsonschema:"description=Extend component functionality with additional features"`

	// DeprecatedScripts are custom commands that run before or after package deployment
	DeprecatedScripts DeprecatedZarfComponentScripts `json:"scripts,omitempty" jsonschema:"description=[Deprecated] (replaced by actions) Custom commands to run before or after package deployment.  This will be removed in Zarf v1.0.0.,deprecated=true"`

	// Replaces scripts, fine-grained control over commands to run at various stages of a package lifecycle
	Actions ZarfComponentActions `json:"actions,omitempty" jsonschema:"description=Custom commands to run at various stages of a package lifecycle"`
}

// RequiresCluster returns if the component requires a cluster connection to deploy
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
//
// If the `Required` field is set, it will return that value.
//
// If the `DefaultRequired` feature flag is present, it will return true.
//
// Otherwise, it will return false.
func (c ZarfComponent) IsRequired(ff []FeatureFlag) bool {
	if c.Required != nil {
		return *c.Required
	}

	if slices.Contains(ff, DefaultRequired) {
		return true
	}

	return false
}

// ZarfComponentOnlyTarget filters a component to only show it for a given local OS and cluster.
type ZarfComponentOnlyTarget struct {
	LocalOS string                   `json:"localOS,omitempty" jsonschema:"description=Only deploy component to specified OS,enum=linux,enum=darwin,enum=windows"`
	Cluster ZarfComponentOnlyCluster `json:"cluster,omitempty" jsonschema:"description=Only deploy component to specified clusters"`
	Flavor  string                   `json:"flavor,omitempty" jsonschema:"description=Only include this component when a matching '--flavor' is specified on 'zarf package create'"`
}

// ZarfComponentOnlyCluster represents the architecture and K8s cluster distribution to filter on.
type ZarfComponentOnlyCluster struct {
	Architecture string   `json:"architecture,omitempty" jsonschema:"description=Only create and deploy to clusters of the given architecture,enum=amd64,enum=arm64"`
	Distros      []string `json:"distros,omitempty" jsonschema:"description=A list of kubernetes distros this package works with (Reserved for future use),example=k3s,example=eks"`
}

// ZarfFile defines a file to deploy.
type ZarfFile struct {
	Source      string   `json:"source" jsonschema:"description=Local folder or file path or remote URL to pull into the package"`
	Shasum      string   `json:"shasum,omitempty" jsonschema:"description=(files only) Optional SHA256 checksum of the file"`
	Target      string   `json:"target" jsonschema:"description=The absolute or relative path where the file or folder should be copied to during package deploy"`
	Executable  bool     `json:"executable,omitempty" jsonschema:"description=(files only) Determines if the file should be made executable during package deploy"`
	Symlinks    []string `json:"symlinks,omitempty" jsonschema:"description=List of symlinks to create during package deploy"`
	ExtractPath string   `json:"extractPath,omitempty" jsonschema:"description=Local folder or file to be extracted from a 'source' archive"`
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	Name        string              `json:"name" jsonschema:"description=The name of the chart within Zarf; note that this must be unique and does not need to be the same as the name in the chart repo"`
	Version     string              `json:"version,omitempty" jsonschema:"description=The version of the chart to deploy; for git-based charts this is also the tag of the git repo by default (when not using the '@' syntax for 'repos')"`
	URL         string              `json:"url,omitempty" jsonschema:"example=OCI registry: oci://ghcr.io/stefanprodan/charts/podinfo,example=helm chart repo: https://stefanprodan.github.io/podinfo,example=git repo: https://github.com/stefanprodan/podinfo (note the '@' syntax for 'repos' is supported here too)" jsonschema_description:"The URL of the OCI registry, chart repository, or git repo where the helm chart is stored"`
	RepoName    string              `json:"repoName,omitempty" jsonschema:"description=The name of a chart within a Helm repository (defaults to the Zarf name of the chart)"`
	GitPath     string              `json:"gitPath,omitempty" jsonschema:"description=(git repo only) The sub directory to the chart within a git repo,example=charts/your-chart"`
	LocalPath   string              `json:"localPath,omitempty" jsonschema:"description=The path to a local chart's folder or .tgz archive"`
	Namespace   string              `json:"namespace" jsonschema:"description=The namespace to deploy the chart to"`
	ReleaseName string              `json:"releaseName,omitempty" jsonschema:"description=The name of the Helm release to create (defaults to the Zarf name of the chart)"`
	NoWait      bool                `json:"noWait,omitempty" jsonschema:"description=Whether to not wait for chart resources to be ready before continuing"`
	ValuesFiles []string            `json:"valuesFiles,omitempty" jsonschema:"description=List of local values file paths or remote URLs to include in the package; these will be merged together when deployed"`
	Variables   []ZarfChartVariable `json:"variables,omitempty" jsonschema:"description=[alpha] List of variables to set in the Helm chart"`
}

// ZarfChartVariable represents a variable that can be set for a Helm chart overrides.
type ZarfChartVariable struct {
	Name        string `json:"name" jsonschema:"description=The name of the variable,pattern=^[A-Z0-9_]+$"`
	Description string `json:"description" jsonschema:"description=A brief description of what the variable controls"`
	Path        string `json:"path" jsonschema:"description=The path within the Helm chart values where this variable applies"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart.
type ZarfManifest struct {
	Name                       string   `json:"name" jsonschema:"description=A name to give this collection of manifests; this will become the name of the dynamically-created helm chart"`
	Namespace                  string   `json:"namespace,omitempty" jsonschema:"description=The namespace to deploy the manifests to"`
	Files                      []string `json:"files,omitempty" jsonschema:"description=List of local K8s YAML files or remote URLs to deploy (in order)"`
	KustomizeAllowAnyDirectory bool     `json:"kustomizeAllowAnyDirectory,omitempty" jsonschema:"description=Allow traversing directory above the current directory if needed for kustomization"`
	Kustomizations             []string `json:"kustomizations,omitempty" jsonschema:"description=List of local kustomization paths or remote URLs to include in the package"`
	NoWait                     bool     `json:"noWait,omitempty" jsonschema:"description=Whether to not wait for manifest resources to be ready before continuing"`
}

// DeprecatedZarfComponentScripts are scripts that run before or after a component is deployed
type DeprecatedZarfComponentScripts struct {
	ShowOutput     bool     `json:"showOutput,omitempty" jsonschema:"description=Show the output of the script during package deployment"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" jsonschema:"description=Timeout in seconds for the script"`
	Retry          bool     `json:"retry,omitempty" jsonschema:"description=Retry the script if it fails"`
	Prepare        []string `json:"prepare,omitempty" jsonschema:"description=Scripts to run before the component is added during package create"`
	Before         []string `json:"before,omitempty" jsonschema:"description=Scripts to run before the component is deployed"`
	After          []string `json:"after,omitempty" jsonschema:"description=Scripts to run after the component successfully deploys"`
}

// ZarfComponentActions are ActionSets that map to different zarf package operations
type ZarfComponentActions struct {
	OnCreate ZarfComponentActionSet `json:"onCreate,omitempty" jsonschema:"description=Actions to run during package creation"`
	OnDeploy ZarfComponentActionSet `json:"onDeploy,omitempty" jsonschema:"description=Actions to run during package deployment"`
	OnRemove ZarfComponentActionSet `json:"onRemove,omitempty" jsonschema:"description=Actions to run during package removal"`
}

// ZarfComponentActionSet is a set of actions to run during a zarf package operation
type ZarfComponentActionSet struct {
	Defaults  ZarfComponentActionDefaults `json:"defaults,omitempty" jsonschema:"description=Default configuration for all actions in this set"`
	Before    []ZarfComponentAction       `json:"before,omitempty" jsonschema:"description=Actions to run at the start of an operation"`
	After     []ZarfComponentAction       `json:"after,omitempty" jsonschema:"description=Actions to run at the end of an operation"`
	OnSuccess []ZarfComponentAction       `json:"onSuccess,omitempty" jsonschema:"description=Actions to run if all operations succeed"`
	OnFailure []ZarfComponentAction       `json:"onFailure,omitempty" jsonschema:"description=Actions to run if all operations fail"`
}

// ZarfComponentActionDefaults sets the default configs for child actions
type ZarfComponentActionDefaults struct {
	Mute            bool       `json:"mute,omitempty" jsonschema:"description=Hide the output of commands during execution (default false)"`
	MaxTotalSeconds int        `json:"maxTotalSeconds,omitempty" jsonschema:"description=Default timeout in seconds for commands (default to 0, no timeout)"`
	MaxRetries      int        `json:"maxRetries,omitempty" jsonschema:"description=Retry commands given number of times if they fail (default 0)"`
	Dir             string     `json:"dir,omitempty" jsonschema:"description=Working directory for commands (default CWD)"`
	Env             []string   `json:"env,omitempty" jsonschema:"description=Additional environment variables for commands"`
	Shell           exec.Shell `json:"shell,omitempty" jsonschema:"description=(cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems"`
}

// ZarfComponentAction represents a single action to run during a zarf package operation
type ZarfComponentAction struct {
	Mute                  *bool                    `json:"mute,omitempty" jsonschema:"description=Hide the output of the command during package deployment (default false)"`
	MaxTotalSeconds       *int                     `json:"maxTotalSeconds,omitempty" jsonschema:"description=Timeout in seconds for the command (default to 0, no timeout for cmd actions and 300, 5 minutes for wait actions)"`
	MaxRetries            *int                     `json:"maxRetries,omitempty" jsonschema:"description=Retry the command if it fails up to given number of times (default 0)"`
	Dir                   *string                  `json:"dir,omitempty" jsonschema:"description=The working directory to run the command in (default is CWD)"`
	Env                   []string                 `json:"env,omitempty" jsonschema:"description=Additional environment variables to set for the command"`
	Cmd                   string                   `json:"cmd,omitempty" jsonschema:"description=The command to run. Must specify either cmd or wait for the action to do anything."`
	Shell                 *exec.Shell              `json:"shell,omitempty" jsonschema:"description=(cmd only) Indicates a preference for a shell for the provided cmd to be executed in on supported operating systems"`
	DeprecatedSetVariable string                   `json:"setVariable,omitempty" jsonschema:"description=[Deprecated] (replaced by setVariables) (onDeploy/cmd only) The name of a variable to update with the output of the command. This variable will be available to all remaining actions and components in the package. This will be removed in Zarf v1.0.0,pattern=^[A-Z0-9_]+$"`
	SetVariables          []variables.Variable     `json:"setVariables,omitempty" jsonschema:"description=(onDeploy/cmd only) An array of variables to update with the output of the command. These variables will be available to all remaining actions and components in the package."`
	Description           string                   `json:"description,omitempty" jsonschema:"description=Description of the action to be displayed during package execution instead of the command"`
	Wait                  *ZarfComponentActionWait `json:"wait,omitempty" jsonschema:"description=Wait for a condition to be met before continuing. Must specify either cmd or wait for the action. See the 'zarf tools wait-for' command for more info."`
}

// ZarfComponentActionWait specifies a condition to wait for before continuing
type ZarfComponentActionWait struct {
	Cluster *ZarfComponentActionWaitCluster `json:"cluster,omitempty" jsonschema:"description=Wait for a condition to be met in the cluster before continuing. Only one of cluster or network can be specified."`
	Network *ZarfComponentActionWaitNetwork `json:"network,omitempty" jsonschema:"description=Wait for a condition to be met on the network before continuing. Only one of cluster or network can be specified."`
}

// ZarfComponentActionWaitCluster specifies a condition to wait for before continuing
type ZarfComponentActionWaitCluster struct {
	Kind       string `json:"kind" jsonschema:"description=The kind of resource to wait for,example=Pod,example=Deployment)"`
	Identifier string `json:"name" jsonschema:"description=The name of the resource or selector to wait for,example=podinfo,example=app&#61;podinfo"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"description=The namespace of the resource to wait for"`
	Condition  string `json:"condition,omitempty" jsonschema:"description=The condition or jsonpath state to wait for; defaults to exist, a special condition that will wait for the resource to exist,example=Ready,example=Available,'{.status.availableReplicas}'=23"`
}

// ZarfComponentActionWaitNetwork specifies a condition to wait for before continuing
type ZarfComponentActionWaitNetwork struct {
	Protocol string `json:"protocol" jsonschema:"description=The protocol to wait for,enum=tcp,enum=http,enum=https"`
	Address  string `json:"address" jsonschema:"description=The address to wait for,example=localhost:8080,example=1.1.1.1"`
	Code     int    `json:"code,omitempty" jsonschema:"description=The HTTP status code to wait for if using http or https,example=200,example=404"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection,example=app&#61;data-injection"`
	Container string `json:"container" jsonschema:"description=The container name to target for data injection"`
	Path      string `json:"path" jsonschema:"description=The path within the container to copy the data into"`
}

// ZarfDataInjection is a data-injection definition.
type ZarfDataInjection struct {
	Source   string              `json:"source" jsonschema:"description=Either a path to a local folder/file or a remote URL of a file to inject into the given target pod + container"`
	Target   ZarfContainerTarget `json:"target" jsonschema:"description=The target pod + container to inject the data into"`
	Compress bool                `json:"compress,omitempty" jsonschema:"description=Compress the data before transmitting using gzip.  Note: this requires support for tar/gzip locally and in the target image."`
}

// ZarfComponentImport structure for including imported Zarf components.
type ZarfComponentImport struct {
	ComponentName string `json:"name,omitempty" jsonschema:"description=The name of the component to import from the referenced zarf.yaml"`
	// For further explanation see https://regex101.com/r/nxX8vx/1
	Path string `json:"path,omitempty" jsonschema:"description=The relative path to a directory containing a zarf.yaml to import from"`
	// For further explanation see https://regex101.com/r/nxX8vx/1
	URL string `json:"url,omitempty" jsonschema:"description=[beta] The URL to a Zarf package to import via OCI,pattern=^oci://.*$"`
}
