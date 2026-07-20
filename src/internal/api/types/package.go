// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types holds the internal generic representation of a Zarf package used for lossless conversions between API versions.
// This type is never exposed publicly. Each API version converts to/from this type, giving N conversion functions instead of N².
// The shape mirrors the latest schema (v1beta1) with extra fields appended where earlier versions carry data that does not survive
// untouched on the latest schema.
package types

// Package is the internal superset representation used for conversions between API versions.
type Package struct {
	APIVersion    string
	Kind          string
	Metadata      PackageMetadata
	Build         BuildData
	Components    []Component
	Values        Values
	Documentation map[string]string

	// v1alpha1-only fields preserved for lossless round-trip.
	Variables []InteractiveVariable
	Constants []Constant
}

// PackageMetadata is the superset of metadata fields across API versions.
type PackageMetadata struct {
	Name         string
	Description  string
	Version      string
	Uncompressed bool
	Architecture string
	Annotations  map[string]string
	// PreventNamespaceOverride is the v1beta1 form. v1alpha1 stores AllowNamespaceOverride *bool;
	// only one of these should be populated by the converter.
	PreventNamespaceOverride bool
	AllowNamespaceOverride   *bool

	// v1alpha1-only metadata fields. v1beta1 migrates these to Annotations.
	URL           string
	Image         string
	YOLO          bool
	Authors       string
	Documentation string
	Source        string
	Vendor        string
	// AggregateChecksum lives in Metadata on v1alpha1 and in Build on v1beta1.
	AggregateChecksum string
}

// BuildData is the superset of build fields across API versions.
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
	// OriginalAPIVersion tracks the apiVersion the package was read from before any conversion.
	OriginalAPIVersion string

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

// Component is the superset of component fields across API versions.
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

	// v1alpha1-only fields preserved for lossless round-trip.
	Default           bool
	Required          *bool
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
	// v1beta1 form: separate lists of local and remote component config references.
	Local  []ComponentImportLocal
	Remote []ComponentImportRemote

	// v1alpha1-only single-import fields.
	Name string
	Path string
	URL  string
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

// Manifest is the superset of manifest fields across API versions.
type Manifest struct {
	Name             string
	Namespace        string
	Files            []string
	Kustomize        *KustomizeManifest
	SkipWait         bool
	ServerSideApply  string
	EnableTemplating bool

	// v1alpha1-only round-trip fields.
	Template *bool
}

// Chart is the superset of chart fields across API versions.
type Chart struct {
	Name                 string
	Namespace            string
	ReleaseName          string
	ValuesFiles          []ValuesFile
	Values               []ChartValue
	SkipSchemaValidation bool
	ServerSideApply      string
	SkipWait             bool

	// v1beta1 structured sources.
	HelmRepository *HelmRepositorySource
	Git            *GitSource
	Local          *LocalSource
	OCI            *OCISource

	// v1alpha1-only flat source fields. Used during conversion to populate structured sources.
	URL              string
	RepoName         string
	GitPath          string
	LocalPath        string
	Version          string
	SchemaValidation *bool
	Variables        []ZarfChartVariable
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
	URL     string
	Version string
	Ref     *OCIRef
}

// Repository defines a git repository.
type Repository struct {
	URL string
	Ref *GitRef
}

// File is the superset of file fields across API versions.
type File struct {
	Source           string
	Checksum         string
	Destination      string
	Executable       bool
	Symlinks         []string
	ExtractPath      string
	EnableTemplating bool
	// Template is the v1alpha1 *bool preserved so an unset value round-trips losslessly.
	Template *bool
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

// ComponentActions are ActionSets mapped to package lifecycle operations.
type ComponentActions struct {
	OnCreate ComponentActionSet
	OnDeploy ComponentActionSet
	OnRemove ComponentActionSet
}

// ComponentActionSet is a set of actions for one lifecycle operation.
type ComponentActionSet struct {
	Defaults  ComponentActionDefaults
	Before    []ComponentAction
	OnSuccess []ComponentAction
	OnFailure []ComponentAction

	// After is the v1alpha1-only "run at the end of an operation" hook, preserved for lossless
	// round-trips. v1beta1 has no equivalent and folds these into OnSuccess on conversion.
	After []ComponentAction
}

// ComponentActionDefaults sets defaults for child actions.
type ComponentActionDefaults struct {
	Silent          bool
	MaxTotalSeconds int32
	Retries         int32
	Dir             string
	Env             []string
	Shell           Shell
}

// ComponentAction is the superset of action fields across API versions.
type ComponentAction struct {
	Silent           *bool
	MaxTotalSeconds  *int32
	Retries          *int32
	Dir              *string
	Env              []string
	Cmd              string
	Shell            *Shell
	SetValues        []SetValue
	Description      string
	Wait             *ComponentActionWait
	EnableTemplating bool

	// v1alpha1-only round-trip fields.
	SetVariables          []Variable
	DeprecatedSetVariable string
	// Template is the v1alpha1 *bool preserved so an unset value round-trips losslessly.
	Template *bool
}

// SetValue declares a value that can be set during a deploy.
type SetValue struct {
	Key   string
	Value any
	Type  string
}

// ComponentActionWait specifies a condition to wait for before continuing.
type ComponentActionWait struct {
	Cluster *ComponentActionWaitCluster
	Network *ComponentActionWaitNetwork
}

// ComponentActionWaitCluster specifies a cluster-level wait condition.
type ComponentActionWaitCluster struct {
	Kind      string
	Name      string
	Namespace string
	Condition string
}

// ComponentActionWaitNetwork specifies a network-level wait condition.
type ComponentActionWaitNetwork struct {
	Protocol string
	Address  string
	Code     int32
}

// Shell represents shell preferences per OS.
type Shell struct {
	Windows string
	Linux   string
	Darwin  string
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
