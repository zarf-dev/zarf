// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types holds the internal generic representation of a Zarf package used for lossless conversions between API versions.
// This type is never exposed publicly. Each API version converts to/from this type, giving N conversion functions instead of N².
package types

import (
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZarfPackage is the internal superset representation used for conversions between API versions.
type ZarfPackage struct {
	APIVersion    string
	Kind          string
	Metadata      ZarfMetadata
	Build         ZarfBuildData
	Components    []ZarfComponent
	Constants     []Constant
	Variables     []InteractiveVariable
	Values        ZarfValues
	Documentation map[string]string
}

// ZarfMetadata is a superset of all metadata fields across API versions.
type ZarfMetadata struct {
	Name                   string
	Description            string
	Version                string
	Uncompressed           bool
	Architecture           string
	Annotations            map[string]string
	AllowNamespaceOverride *bool

	// v1alpha1-only fields
	URL           string
	Image         string
	YOLO          bool
	Authors       string
	Documentation string
	Source        string
	Vendor        string
	// AggregateChecksum lives in metadata in v1alpha1, build in v1beta1
	AggregateChecksum string
}

// ZarfBuildData is a superset of all build fields across API versions.
type ZarfBuildData struct {
	Terminal                   string
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
	APIVersion                 string

	// v1alpha1-only
	DifferentialMissing []string
}

// VersionRequirement specifies a minimum Zarf version needed.
type VersionRequirement struct {
	Version string
	Reason  string
}

// Constant represents a template constant.
type Constant struct {
	Name        string
	Value       string
	Description string
	AutoIndent  bool
	Pattern     string
}

// Variable represents a variable.
type Variable struct {
	Name       string
	Sensitive  bool
	AutoIndent bool
	Pattern    string
	Type       string
}

// InteractiveVariable is a variable that can prompt the user.
type InteractiveVariable struct {
	Variable
	Description string
	Default     string
	Prompt      bool
}

// SetVariable tracks a variable that has been set.
type SetVariable struct {
	Variable
	Value string
}

// SetValue declares a value that can be set during deploy.
type SetValue struct {
	Key   string
	Value any
	Type  string
}

// ZarfValues defines values files and schema.
type ZarfValues struct {
	Files  []string
	Schema string
}

// ZarfComponent is the internal superset of component fields.
type ZarfComponent struct {
	Name          string
	Description   string
	Only          ZarfComponentOnlyTarget
	Import        ZarfComponentImport
	Manifests     []ZarfManifest
	Charts        []ZarfChart
	Files         []ZarfFile
	Images        []ZarfImage
	ImageArchives []ImageArchive
	Repos         []string
	Actions       ZarfComponentActions
	Features      ZarfComponentFeatures

	// v1alpha1-only fields preserved for lossless conversion
	Default        bool
	Required       *bool
	Optional       *bool
	Group          string
	DataInjections []v1alpha1.ZarfDataInjection
	HealthChecks   []v1alpha1.NamespacedObjectKindReference
	// v1alpha1 chart variables are dropped (no shim needed, per proposal)
}

// ZarfComponentOnlyTarget filters a component.
type ZarfComponentOnlyTarget struct {
	LocalOS string
	Cluster ZarfComponentOnlyCluster
	Flavor  string
}

// ZarfComponentOnlyCluster represents architecture and distro filters.
type ZarfComponentOnlyCluster struct {
	Architecture string
	Distros      []string
}

// ZarfComponentImport defines an imported component.
type ZarfComponentImport struct {
	// v1alpha1-only
	Name string
	Path string
	URL  string
}

// ZarfFile defines a file to deploy.
type ZarfFile struct {
	Source      string
	Shasum      string
	Target      string
	Executable  bool
	Symlinks    []string
	ExtractPath string
	Template    *bool
}

// ImageSource represents where an image is pulled from.
type ImageSource string

const (
	// ImageSourceRegistry pulls from an OCI registry.
	ImageSourceRegistry ImageSource = "registry"
	// ImageSourceDaemon pulls from the local Docker daemon.
	ImageSourceDaemon ImageSource = "daemon"
)

// ZarfImage represents an OCI image.
type ZarfImage struct {
	Name   string
	Source ImageSource
}

// GetSource returns the image source, defaulting to ImageSourceRegistry.
func (img ZarfImage) GetSource() ImageSource {
	if img.Source == "" {
		return ImageSourceRegistry
	}
	return img.Source
}

// ImageArchive defines a tar archive of images.
type ImageArchive struct {
	Path   string
	Images []string
}

// ZarfChart is the internal superset of chart fields.
type ZarfChart struct {
	Name             string
	Namespace        string
	ReleaseName      string
	ValuesFiles      []string
	Values           []ZarfChartValue
	SchemaValidation *bool
	ServerSideApply  string
	Wait             *bool

	// v1beta1 structured sources
	HelmRepo HelmRepoSource
	Git      GitRepoSource
	Local    LocalRepoSource
	OCI      OCISource

	// v1alpha1 flat fields (used during conversion to populate structured sources)
	URL       string
	RepoName  string
	GitPath   string
	LocalPath string
	Version   string
	NoWait    bool
}

// ZarfChartValue maps a source path to a target path.
type ZarfChartValue struct {
	SourcePath string
	TargetPath string
}

// HelmRepoSource represents a Helm chart in a repository.
type HelmRepoSource struct {
	Name    string
	URL     string
	Version string
}

// GitRepoSource represents a Helm chart in a Git repo.
type GitRepoSource struct {
	URL  string
	Path string
}

// LocalRepoSource represents a local Helm chart.
type LocalRepoSource struct {
	Path string
}

// OCISource represents a Helm chart in an OCI registry.
type OCISource struct {
	URL     string
	Version string
}

// ZarfManifest defines raw manifests deployed as a Helm chart.
type ZarfManifest struct {
	Name                       string
	Namespace                  string
	Files                      []string
	KustomizeAllowAnyDirectory bool
	Kustomizations             []string
	ServerSideApply            string
	Template                   *bool
	Wait                       *bool
	NoWait                     bool

	// v1alpha1-only
	EnableKustomizePlugins bool
}

// ZarfComponentFeatures defines CLI features for a component.
type ZarfComponentFeatures struct {
	IsRegistry bool
	Injector   *Injector
	IsAgent    bool
}

// Injector defines the Zarf injector configuration.
type Injector struct {
	Enabled bool
	Values  *InjectorValues
}

// InjectorValues defines configurable values for the injector.
type InjectorValues struct {
	Tolerations string
}

// ZarfComponentActions are action sets mapped to lifecycle operations.
type ZarfComponentActions struct {
	OnCreate ZarfComponentActionSet
	OnDeploy ZarfComponentActionSet
	OnRemove ZarfComponentActionSet
}

// ZarfComponentActionSet is a set of actions for a lifecycle operation.
type ZarfComponentActionSet struct {
	Defaults  ZarfComponentActionDefaults
	Before    []ZarfComponentAction
	After     []ZarfComponentAction
	OnSuccess []ZarfComponentAction // v1alpha1-only, merged into After for v1beta1
	OnFailure []ZarfComponentAction
}

// ZarfComponentActionDefaults sets default configs for child actions.
type ZarfComponentActionDefaults struct {
	Mute    bool
	Timeout *metav1.Duration
	Retries int
	Dir     string
	Env     []string
	Shell   Shell

	// v1alpha1 fields
	MaxTotalSeconds int
	MaxRetries      int
}

// ZarfComponentAction represents a single action.
type ZarfComponentAction struct {
	Mute         *bool
	Timeout      *metav1.Duration
	Retries      int
	Dir          *string
	Env          []string
	Cmd          string
	Shell        *Shell
	SetVariables []Variable
	SetValues    []SetValue
	Description  string
	Wait         *ZarfComponentActionWait
	Template     *bool

	// v1alpha1 fields
	MaxTotalSeconds       *int
	MaxRetries            *int
	DeprecatedSetVariable string
}

// ZarfComponentActionWait specifies a wait condition.
type ZarfComponentActionWait struct {
	Cluster *ZarfComponentActionWaitCluster
	Network *ZarfComponentActionWaitNetwork
}

// ZarfComponentActionWaitCluster specifies a cluster wait condition.
type ZarfComponentActionWaitCluster struct {
	Kind      string
	Name      string
	Namespace string
	Condition string
}

// ZarfComponentActionWaitNetwork specifies a network wait condition.
type ZarfComponentActionWaitNetwork struct {
	Protocol string
	Address  string
	Code     int
}

// Shell represents shell preferences per OS.
type Shell struct {
	Windows string
	Linux   string
	Darwin  string
}
