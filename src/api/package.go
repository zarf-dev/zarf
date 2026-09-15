// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package api defines Zarf's version-neutral package model.
package api

// Package is the version-neutral representation used by package operations and converters.
type Package struct {
	// APIVersion identifies the source package schema. An empty value is the legacy v1alpha1 form.
	APIVersion    string
	Kind          PackageKind
	Metadata      PackageMetadata
	Build         BuildData
	Components    []Component
	Values        Values
	Documentation map[string]string

	// v1alpha1-only fields
	Variables []InteractiveVariable
	Constants []Constant
}

// PackageMetadata contains metadata shared across package API versions.
type PackageMetadata struct {
	Name                     string
	Description              string
	Version                  string
	Uncompressed             bool
	Architecture             string
	Annotations              map[string]string
	PreventNamespaceOverride bool

	// YOLO changes deploy behavior for v1alpha1 packages.
	YOLO bool
}

// BuildData contains build metadata shared across package API versions.
type BuildData struct {
	// Hostname is the v1beta1 name (v1alpha1: Terminal).
	Hostname                   string
	User                       string
	Architecture               string
	Timestamp                  string
	Version                    string
	Migrations                 []string
	RegistryOverrides          map[string]string
	Differential               bool
	DifferentialPackageVersion string
	Flavor                     string
	Signed                     *bool
	VersionRequirements        []VersionRequirement
	ProvenanceFiles            []string
	AggregateChecksum          string

	// v1alpha1-only build fields.
	DifferentialMissing []string
}

// VersionRequirement specifies a minimum Zarf version needed.
type VersionRequirement struct {
	Version string
	Reason  string
}

// Values defines values files and schema.
type Values struct {
	Files  []string
	Schema string
}

// Component is the version-neutral representation of a package component.
type Component struct {
	Name          string
	Description   string
	Optional      bool
	Target        ComponentTarget
	Import        ComponentImport
	Service       string
	Manifests     []Manifest
	Charts        []Chart
	Files         []File
	Images        []Image
	ImageArchives []ImageArchive
	Repositories  []Repository
	StateAccess   []string
	Actions       ComponentActions

	Default           bool
	Group             string
	DataInjections    []ZarfDataInjection
	HealthChecks      []NamespacedObjectKindReference
	Distros           []string
	DeprecatedScripts DeprecatedComponentScripts
}

// DeprecatedComponentScripts is the v1alpha1-only pre-actions scripts block, preserved for lossless
// round-trip.
type DeprecatedComponentScripts struct {
	ShowOutput     bool
	TimeoutSeconds int
	Retry          bool
	Prepare        []string
	Before         []string
	After          []string
}

// ComponentTarget filters a component to a target OS/arch/flavor.
type ComponentTarget struct {
	OS           string
	Architecture string
	Flavor       string
}

// ComponentImport carries imports from any API version.
type ComponentImport struct {
	// Local and Remote support the multiple imports accepted by v1beta1.
	Local  []ComponentImportLocal
	Remote []ComponentImportRemote

	// Name identifies a v1alpha1 imported component. Path and URL are projected onto
	// Local and Remote by the v1alpha1 converter.
	Name string
}

// ComponentImportLocal references a local component config file.
type ComponentImportLocal struct {
	Path string
}

// ComponentImportRemote references a remote (OCI) component config.
type ComponentImportRemote struct {
	URL string
}

// KustomizeManifest holds kustomization settings for a manifest.
type KustomizeManifest struct {
	Files             []string
	AllowAnyDirectory bool
	EnablePlugins     bool
}

// Manifest is the version-neutral representation of a manifest entry.
type Manifest struct {
	Name             string
	Namespace        string
	Files            []string
	Kustomize        *KustomizeManifest
	SkipWait         bool
	ServerSideApply  string
	EnableTemplating bool
}

// Chart is the operational representation of a chart across API versions.
type Chart struct {
	Name string
	// Version identifies this chart's archive and values files within the package.
	Version              string
	Namespace            string
	ReleaseName          string
	ValuesFiles          []ValuesFile
	Values               []ChartValue
	SkipSchemaValidation bool
	ServerSideApply      string
	SkipWait             bool

	HelmRepository *HelmRepositorySource
	Git            *GitSource
	Local          *LocalSource
	OCI            *OCISource

	// Variables are required to run v1alpha1 chart actions.
	Variables []ZarfChartVariable
}

// ValuesFile is a values file merged into a Helm chart, optionally rendered with Zarf templating.
type ValuesFile struct {
	Path             string
	EnableTemplating bool
}

// ChartValue maps a source path to a target path.
type ChartValue struct {
	SourcePath   string
	TargetPath   string
	ExcludePaths []string
}

// HelmRepositorySource represents a chart stored in a Helm repository.
type HelmRepositorySource struct {
	Name    string
	URL     string
	Version string
}

// GitRef selects a single Git reference.
type GitRef struct {
	Tag    string
	Branch string
	Commit string
}

// GitSource represents a chart stored in a Git repository.
type GitSource struct {
	URL  string
	Path string
	Ref  *GitRef
}

// LocalSource represents a chart stored locally.
type LocalSource struct {
	Path string
}

// OCIRef selects a single OCI reference.
type OCIRef struct {
	Tag    string
	Digest string
}

// OCISource represents a chart stored in an OCI registry.
type OCISource struct {
	URL string
	Ref *OCIRef
}

// Repository defines a git repository.
type Repository struct {
	URL string
	Ref *GitRef
}

// File is the version-neutral representation of a package file.
type File struct {
	Source           string
	Checksum         string
	Destination      string
	Executable       bool
	Symlinks         []string
	ExtractPath      string
	EnableTemplating bool
}

// Image represents an OCI image in the package.
type Image struct {
	Name   string
	Source string
}

// ImageArchive defines a tar archive of images to include in the package.
type ImageArchive struct {
	Path   string
	Images []string
}

// ComponentActions are the actions associated with each package lifecycle operation.
type ComponentActions struct {
	OnCreate ActionSet
	OnDeploy ActionSet
	OnRemove ActionSet
}

// ActionSet contains actions for one package lifecycle operation.
type ActionSet struct {
	Defaults  ActionDefaults
	Before    []Action
	After     []Action
	OnSuccess []Action
	OnFailure []Action
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
	// DeprecatedSetVariable is required to execute legacy v1alpha1 packages.
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

// VariableType represents a type of a Zarf package variable.
type VariableType string

const (
	// RawVariableType is the default type for a Zarf package variable.
	RawVariableType VariableType = "raw"
	// FileVariableType loads a variable's contents from a file.
	FileVariableType VariableType = "file"
)

// Variable represents a variable that has a value set programmatically.
type Variable struct {
	Name       string
	Sensitive  bool
	AutoIndent bool
	Pattern    string
	Type       VariableType
}

// InteractiveVariable is a variable that can prompt a user for more information.
type InteractiveVariable struct {
	Variable
	Description string
	Default     string
	Prompt      bool
}

// Constant is a value that can be used to dynamically template resources or run in actions.
type Constant struct {
	Name        string
	Value       string
	Description string
	AutoIndent  bool
	Pattern     string
}

// ZarfChartVariable represents a variable that can be set for Helm chart overrides.
type ZarfChartVariable struct {
	Name        string
	Description string
	Path        string
}

// ZarfContainerTarget defines the destination info for a ZarfDataInjection target.
type ZarfContainerTarget struct {
	Namespace string
	Selector  string
	Container string
	Path      string
}

// ZarfDataInjection is a data-injection definition.
type ZarfDataInjection struct {
	Source   string
	Target   ZarfContainerTarget
	Compress bool
}

// NamespacedObjectKindReference references a cluster resource targeted by a health check.
type NamespacedObjectKindReference struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}
