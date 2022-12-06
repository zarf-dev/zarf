// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf
package types

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	Confirm       bool   `json:"confirm" jsonschema:"description=Verify that Zarf should perform an action"`
	CachePath     string `json:"cachePath" jsonschema:"description=Path to use to cache images and git repos on package create"`
	TempDirectory string `json:"tempDirectory" jsonschema:"description=Location Zarf should use as a staging ground when managing files and images for package creation and deployment"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment
type ZarfDeployOptions struct {
	Insecure     bool              `json:"insecure" jsonschema:"description=Allow insecure connections for remote packages"`
	Shasum       string            `json:"shasum" jsonschema:"description=The SHA256 checksum of the package to deploy"`
	PackagePath  string            `json:"packagePath" jsonschema:"description=Location where a Zarf package to deploy can be found"`
	Components   string            `json:"components" jsonschema:"description=Comma separated list of optional components to deploy"`
	SGetKeyPath  string            `json:"sGetKeyPath" jsonschema:"description=Location where the public key component of a cosign key-pair can be found"`
	SetVariables map[string]string `json:"setVariables" jsonschema:"description=Key-Value map of variable names and their corresponding values that will be used to template against the Zarf package being used"`
}

// ZarfInitOptions tracks the user-defined options during cluster initialization.
type ZarfInitOptions struct {
	// Zarf init is installing the k3s component
	ApplianceMode bool `json:"applianceMode" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`

	// Using a remote git server
	GitServer GitServerInfo `json:"gitServer" jsonschema:"description=Information about the repository Zarf is going to be using"`

	RegistryInfo RegistryInfo `json:"registryInfo" jsonschema:"description=Information about the registry Zarf is going to be using"`

	Components string `json:"components" jsonschema:"description=Comma separated list of optional components to deploy"`

	StorageClass string `json:"storageClass" jsonschema:"description=StorageClass of the k8s cluster Zarf is initializing"`
}

// ZarfCreateOptions tracks the user-defined options used to create the package.
type ZarfCreateOptions struct {
	SkipSBOM        bool              `json:"skipSBOM" jsonschema:"description=Disable the generation of SBOM materials during package creation"`
	Insecure        bool              `json:"insecure" jsonschema:"description=Disable the need for shasum validations when pulling down files from the internet"`
	OutputDirectory string            `json:"outputDirectory" jsonschema:"description=Location where the finalized Zarf package will be placed"`
	SBOM            bool              `json:"sbom" jsonschema:"description=Whether to pause to allow for viewing the SBOM post-creation"`
	SBOMOutput      string            `json:"sbomOutput" jsonschema:"description=Location to output an SBOM into after package creation"`
	SetVariables    map[string]string `json:"setVariables" jsonschema:"description=Key-Value map of variable names and their corresponding values that will be used to template against the Zarf package being used"`
}

type ConnectString struct {
	Description string `json:"description" jsonschema:"description=Descriptive text that explains what the resource you would be connecting to is used for"`
	Url         string `json:"url" jsonschema:"description=URL path that gets appended to the k8s port-forward result"`
}

type ConnectStrings map[string]ConnectString

type ComponentPaths struct {
	Base           string
	Files          string
	Charts         string
	Values         string
	Repos          string
	Manifests      string
	DataInjections string
}
type TempPaths struct {
	Base         string
	InjectBinary string
	SeedImage    string
	Images       string
	Components   string
	Sboms        string
	ZarfYaml     string
}
