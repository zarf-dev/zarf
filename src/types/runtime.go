// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
)

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	Confirm       bool   `json:"confirm" jsonschema:"description=Verify that Zarf should perform an action"`
	Insecure      bool   `json:"insecure" jsonschema:"description=Allow insecure connections for remote packages"`
	CachePath     string `json:"cachePath" jsonschema:"description=Path to use to cache images and git repos on package create"`
	TempDirectory string `json:"tempDirectory" jsonschema:"description=Location Zarf should use as a staging ground when managing files and images for package creation and deployment"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment.
type ZarfDeployOptions struct {
	Shasum       string            `json:"shasum" jsonschema:"description=The SHA256 checksum of the package to deploy"`
	PackagePath  string            `json:"packagePath" jsonschema:"description=Location where a Zarf package to deploy can be found"`
	Components   string            `json:"components" jsonschema:"description=Comma separated list of optional components to deploy"`
	SGetKeyPath  string            `json:"sGetKeyPath" jsonschema:"description=Location where the public key component of a cosign key-pair can be found"`
	SetVariables map[string]string `json:"setVariables" jsonschema:"description=Key-Value map of variable names and their corresponding values that will be used to template against the Zarf package being used"`
}

// ZarfPublishOptions tracks the user-defined preferences during a package publish.
type ZarfPublishOptions struct {
	Reference   registry.Reference `jsonschema:"description=Remote registry reference"`
	CopyOptions oras.CopyOptions   `jsonschema:"description=Options for the copy operation"`
	PackOptions oras.PackOptions   `jsonschema:"description=Options for the pack operation"`
	PackagePath string             `json:"packagePath" jsonschema:"description=Location where a Zarf package to publish can be found"`
}

// ZarfInitOptions tracks the user-defined options during cluster initialization.
type ZarfInitOptions struct {
	// Zarf init is installing the k3s component
	ApplianceMode bool `json:"applianceMode" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`

	// Using a remote git server
	GitServer GitServerInfo `json:"gitServer" jsonschema:"description=Information about the repository Zarf is going to be using"`

	RegistryInfo RegistryInfo `json:"registryInfo" jsonschema:"description=Information about the registry Zarf is going to be using"`

	StorageClass string `json:"storageClass" jsonschema:"description=StorageClass of the k8s cluster Zarf is initializing"`
}

// ZarfCreateOptions tracks the user-defined options used to create the package.
type ZarfCreateOptions struct {
	SkipSBOM         bool              `json:"skipSBOM" jsonschema:"description=Disable the generation of SBOM materials during package creation"`
	OutputDirectory  string            `json:"outputDirectory" jsonschema:"description=Location where the finalized Zarf package will be placed"`
	ViewSBOM         bool              `json:"sbom" jsonschema:"description=Whether to pause to allow for viewing the SBOM post-creation"`
	SBOMOutputDir    string            `json:"sbomOutput" jsonschema:"description=Location to output an SBOM into after package creation"`
	SetVariables     map[string]string `json:"setVariables" jsonschema:"description=Key-Value map of variable names and their corresponding values that will be used to template against the Zarf package being used"`
	MaxPackageSizeMB int               `json:"maxPackageSizeMB" jsonschema:"description=Size of chunks to use when splitting a zarf package into multiple files in megabytes"`
}

// ZarfPartialPackageData contains info about a partial package.
type ZarfPartialPackageData struct {
	Sha256Sum string `json:"sha256Sum" jsonschema:"description=The sha256sum of the package"`
	Bytes     int64  `json:"bytes" jsonschema:"description=The size of the package in bytes"`
	Count     int    `json:"count" jsonschema:"description=The number of parts the package is split into"`
}

// ConnectString contains information about a connection made with Zarf connect.
type ConnectString struct {
	Description string `json:"description" jsonschema:"description=Descriptive text that explains what the resource you would be connecting to is used for"`
	URL         string `json:"url" jsonschema:"description=URL path that gets appended to the k8s port-forward result"`
}

// ConnectStrings is a map of connect names to connection information.
type ConnectStrings map[string]ConnectString

// ComponentSBOM contains information related to the files SBOM'ed from a component.
type ComponentSBOM struct {
	Files         []string
	ComponentPath ComponentPaths
}

// ComponentPaths is a struct that represents all of the subdirectories for a Zarf component.
type ComponentPaths struct {
	Base           string
	Temp           string
	Files          string
	Charts         string
	Values         string
	Repos          string
	Manifests      string
	DataInjections string
}

// TempPaths is a struct that represents all of the subdirectories for a Zarf package.
type TempPaths struct {
	Base         string
	InjectBinary string
	SeedImage    string
	Images       string
	Components   string
	SbomTar      string
	ZarfYaml     string
}
