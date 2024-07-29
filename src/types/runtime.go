// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"time"
)

// Zarf looks for these strings in zarf.yaml to make dynamic changes
const (
	ZarfPackageTemplatePrefix = "###ZARF_PKG_TMPL_"
	ZarfPackageVariablePrefix = "###ZARF_PKG_VAR_"
	ZarfPackageArch           = "###ZARF_PKG_ARCH###"
	ZarfComponentName         = "###ZARF_COMPONENT_NAME###"
)

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	// Verify that Zarf should perform an action
	Confirm bool `json:"confirm"`
	// Allow insecure connections for remote packages
	Insecure bool `json:"insecure"`
	// Path to use to cache images and git repos on package create
	CachePath string `json:"cachePath"`
	// Location Zarf should use as a staging ground when managing files and images for package creation and deployment
	TempDirectory string `json:"tempDirectory"`
	// Number of concurrent layer operations to perform when interacting with a remote package
	OCIConcurrency int `json:"ociConcurrency"`
}

// ZarfPackageOptions tracks the user-defined preferences during common package operations.
type ZarfPackageOptions struct {
	// The SHA256 checksum of the package
	Shasum string `json:"shasum"`
	// Location where a Zarf package can be found
	PackageSource string `json:"packageSource"`
	// Comma separated list of optional components
	OptionalComponents string `json:"optionalComponents"`
	// Location where the public key component of a cosign key-pair can be found
	SGetKeyPath string `json:"sGetKeyPath"`
	// Key-Value map of variable names and their corresponding values that will be used to template manifests and files in the Zarf package
	SetVariables map[string]string `json:"setVariables"`
	// Location where the public key component of a cosign key-pair can be found
	PublicKeyPath string `json:"publicKeyPath"`
	// The number of retries to perform for Zarf deploy operations like image pushes or Helm installs
	Retries int `json:"retries"`
}

// ZarfInspectOptions tracks the user-defined preferences during a package inspection.
type ZarfInspectOptions struct {
	// View SBOM contents while inspecting the package
	ViewSBOM bool `json:"viewSBOM"`
	// Location to output an SBOM into after package inspection
	SBOMOutputDir string `json:"sbomOutputDir"`
	// ListImages will list the images in the package
	ListImages bool `json:"listImages"`
}

// ZarfFindImagesOptions tracks the user-defined preferences during a prepare find-images search.
type ZarfFindImagesOptions struct {
	// Path to the helm chart directory
	RepoHelmChartPath string `json:"repoHelmChartPath"`
	// Kubernetes version to use for the helm chart
	KubeVersionOverride string `json:"kubeVersionOverride"`
	// Manual override for ###ZARF_REGISTRY###
	RegistryURL string `json:"registryUrl"`
	// Find the location of the image given as an argument and print it to the console
	Why string `json:"why"`
	// Optionally skip lookup of cosign artifacts when finding images
	SkipCosign bool `json:"skipCosign"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deploy.
type ZarfDeployOptions struct {
	// Whether to adopt any pre-existing K8s resources into the Helm charts managed by Zarf
	AdoptExistingResources bool `json:"adoptExistingResources"`
	// Skip waiting for external webhooks to execute as each package component is deployed
	SkipWebhooks bool `json:"skipWebhooks"`
	// Timeout for performing Helm operations
	Timeout time.Duration `json:"timeout"`
	// [Library Only] A map of component names to chart names containing Helm Chart values to override values on deploy
	ValuesOverridesMap map[string]map[string]map[string]interface{} `json:"valuesOverridesMap"`
}

// ZarfMirrorOptions tracks the user-defined preferences during a package mirror.
type ZarfMirrorOptions struct {
	// Whether to skip adding a Zarf checksum to image references
	NoImgChecksum bool `json:"noImgChecksum"`
}

// ZarfPublishOptions tracks the user-defined preferences during a package publish.
type ZarfPublishOptions struct {
	// Location where the Zarf package will be published to
	PackageDestination string `json:"packageDestination"`
	// Password to the private key signature file that will be used to sign the published package
	SigningKeyPassword string `json:"signingKeyPassword"`
	// Location where the private key component of a cosign key-pair can be found
	SigningKeyPath string `json:"signingKeyPath"`
}

// ZarfPullOptions tracks the user-defined preferences during a package pull.
type ZarfPullOptions struct {
	// Location where the pulled Zarf package will be placed
	OutputDirectory string `json:"outputDirectory"`
}

// ZarfGenerateOptions tracks the user-defined options during package generation.
type ZarfGenerateOptions struct {
	// Name of the package being generated
	Name string `json:"name"`
	// URL to the source git repository
	URL string `json:"url"`
	// Version of the chart to use
	Version string `json:"version"`
	// Relative path to the chart in the git repository
	GitPath string `json:"gitPath"`
	// Location where the finalized zarf.yaml will be placed
	Output string `json:"output"`
}

// ZarfInitOptions tracks the user-defined options during cluster initialization.
type ZarfInitOptions struct {
	// Indicates if Zarf was initialized while deploying its own k8s cluster
	ApplianceMode bool `json:"applianceMode"`
	// Information about the repository Zarf is going to be using
	GitServer GitServerInfo `json:"gitServer"`
	// Information about the container registry Zarf is going to be using
	RegistryInfo RegistryInfo `json:"registryInfo"`
	// Information about the artifact registry Zarf is going to be using
	ArtifactServer ArtifactServerInfo `json:"artifactServer"`
	// StorageClass of the k8s cluster Zarf is initializing
	StorageClass string `json:"storageClass"`
}

// ZarfCreateOptions tracks the user-defined options used to create the package.
type ZarfCreateOptions struct {
	// Disable the generation of SBOM materials during package creation
	SkipSBOM bool `json:"skipSBOM"`
	// Location where the Zarf package will be created from
	BaseDir string `json:"baseDir"`
	// Location where the finalized Zarf package will be placed
	Output string `json:"output"`
	// Whether to pause to allow for viewing the SBOM post-creation
	ViewSBOM bool `json:"viewSBOM"`
	// Location to output an SBOM into after package creation
	SBOMOutputDir string `json:"sbomOutputDir"`
	// Key-Value map of variable names and their corresponding values that will be used to template against the Zarf package being used
	SetVariables map[string]string `json:"setVariables"`
	// Size of chunks to use when splitting a zarf package into multiple files in megabytes
	MaxPackageSizeMB int `json:"maxPackageSizeMB"`
	// Location where the private key component of a cosign key-pair can be found
	SigningKeyPath string `json:"signingKeyPath"`
	// Password to the private key signature file that will be used to sigh the created package
	SigningKeyPassword string `json:"signingKeyPassword"`
	// Path to a previously built package used as the basis for creating a differential package
	DifferentialPackagePath string `json:"differentialPackagePath"`
	// A map of domains to override on package create when pulling images
	RegistryOverrides map[string]string `json:"registryOverrides"`
	// An optional variant that controls which components will be included in a package
	Flavor string `json:"flavor"`
	// Whether to create a skeleton package
	IsSkeleton bool `json:"isSkeleton"`
	// Whether to create a YOLO package
	NoYOLO bool `json:"noYOLO"`
}

// ZarfSplitPackageData contains info about a split package.
type ZarfSplitPackageData struct {
	// The sha256sum of the package
	Sha256Sum string `json:"sha256Sum"`
	// The size of the package in bytes
	Bytes int64 `json:"bytes"`
	// The number of parts the package is split into
	Count int `json:"count"`
}

// ConnectString contains information about a connection made with Zarf connect.
type ConnectString struct {
	// Descriptive text that explains what the resource you would be connecting to is used for
	Description string `json:"description"`
	// URL path that gets appended to the k8s port-forward result
	URL string `json:"url"`
}

// ConnectStrings is a map of connect names to connection information.
type ConnectStrings map[string]ConnectString

// DifferentialData contains image and repository information about the package a Differential Package is Based on.
type DifferentialData struct {
	DifferentialImages         map[string]bool `json:"differentialImages"`
	DifferentialRepos          map[string]bool `json:"differentialRepos"`
	DifferentialPackageVersion string          `json:"differentialPackageVersion"`
}
