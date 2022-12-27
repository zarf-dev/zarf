// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package // Package types contains all the types used by Zarf

package types

// ZarfComponent is the primary functional grouping of assets to deploy by zarf.
type ZarfComponent struct {
	// Name is the unique identifier for this component
	Name string `json:"name" jsonschema:"description=The name of the component,pattern=^[a-z0-9\\-]+$"`

	// Description is a message given to a user when deciding to enable this componenent or not
	Description string `json:"description,omitempty" jsonschema:"description=Message to include during package deploy describing the purpose of this component"`

	// Default changes the default option when deploying this component
	Default bool `json:"default,omitempty" jsonschema:"description=Determines the default Y/N state for installing this component on package deploy"`

	// Required makes this component mandatory for package deployment
	Required bool `json:"required,omitempty" jsonschema:"description=Do not prompt user to install this component, always install on package deploy"`

	// Only include compatible components during package deployment
	Only ZarfComponentOnlyTarget `json:"only,omitempty" jsonschema:"description=Filter when this component is included in package creation or deployment"`

	// Mapping of target envs to produce components for
	OnlyMatrix ZarfComponentOnlyMatrix `json:"matrix,omitempty" jsonschema:"description=Matrix of components to deploy. This will copy the component and create a new component with the given ComponentOnlyTarget"`

	// Key to match other components to produce a user selector field, used to create a BOOLEAN XOR for a set of components
	// Note: ignores default and required flags
	Group string `json:"group,omitempty" jsonschema:"description=Create a user selector field based on all components in the same group"`

	//Path to cosign publickey for signed online resources
	CosignKeyPath string `json:"cosignKeyPath,omitempty" jsonschema:"description=Specify a path to a public key to validate signed online resources"`

	// Import refers to another zarf.yaml package component.
	Import ZarfComponentImport `json:"import,omitempty" jsonschema:"description=Import a component from another Zarf package"`

	// (Deprecated) DeprecatedScripts are custom commands that run before or after package deployment
	DeprecatedScripts DeprecatedZarfComponentScripts `json:"scripts,omitempty" jsonschema:"description=(Deprecated--use actions instead) Custom commands to run before or after package deployment,deprecated=true"`

	// Replaces scripts, fine-grained control over commands to run at various stages of a package lifecycle
	Actions ZarfComponentActions `json:"actions,omitempty" jsonschema:"description=Custom commands to run at various stages of a package lifecycle"`

	// Local or remote hash file to use as a list of files to download/copy, verify and include in the package
	FileHashList []ZarfFileHashList `json:"fileHashList,omitempty" jsonschema:"description=List of files to hash and include in the package"`

	// Files are files to place on disk during deploy
	Files []ZarfFile `json:"files,omitempty" jsonschema:"description=Files to place on disk during package deployment"`

	// Charts are helm charts to install during package deploy
	Charts []ZarfChart `json:"charts,omitempty" jsonschema:"description=Helm charts to install during package deploy"`

	// Manifests are raw manifests that get converted into zarf-generated helm charts during deploy
	Manifests []ZarfManifest `json:"manifests,omitempty"`

	// Images are the online images needed to be included in the zarf package
	Images []string `json:"images,omitempty" jsonschema:"description=List of OCI images to include in the package"`

	// Repos are any git repos that need to be pushed into the git server
	Repos []string `json:"repos,omitempty" jsonschema:"description=List of git repos to include in the package"`

	// Data pacakges to push into a running cluster
	DataInjections []ZarfDataInjection `json:"dataInjections,omitempty" jsonschema:"description=Datasets to inject into a pod in the target cluster"`
}

type ZarfComponentOnlyMatrix struct {
	LocalOS     []string `json:"localOS,omitempty" jsonschema:"description=Only deploy component to specified OS,enum=linux,enum=darwin,enum=windows"`
	LocalArch   []string `json:"localArch,omitempty" jsonschema:"description=Only deploy component to specified architecture,enum=amd64,enum=arm64"`
	ClusterArch []string `json:"clusterArch,omitempty" jsonschema:"description=Only create and deploy to clusters of the given architecture,enum=amd64,enum=arm64"`
}

// ZarfComponentOnlyTarget filters a component to only show it for a given OS/Arch
type ZarfComponentOnlyTarget struct {
	LocalOS   string                   `json:"localOS,omitempty" jsonschema:"description=Only deploy component to specified OS,enum=linux,enum=darwin,enum=windows"`
	LocalArch string                   `json:"localArch,omitempty" jsonschema:"description=Only deploy component to specified architecture,enum=amd64,enum=arm64"`
	Cluster   ZarfComponentOnlyCluster `json:"cluster,omitempty" jsonschema:"description=Only deploy component to specified clusters"`
}

type ZarfComponentOnlyCluster struct {
	Architecture string   `json:"architecture,omitempty" jsonschema:"description=Only create and deploy to clusters of the given architecture,enum=amd64,enum=arm64"`
	Distros      []string `json:"distros,omitempty" jsonschema:"description=Future use"`
}

// ZarfFile defines a file to deploy.
type ZarfFile struct {
	Source     string   `json:"source" jsonschema:"description=Local file path or remote URL to add to the package"`
	Shasum     string   `json:"shasum,omitempty" jsonschema:"description=SHA256 checksum of the file if the source is a URL"`
	Target     string   `json:"target" jsonschema:"description=The absolute or relative path where the file should be copied to during package deploy"`
	Executable bool     `json:"executable,omitempty" jsonschema:"description=Determines if the file should be made executable during package deploy"`
	Extract    string   `json:"extract,omitempty" jsonschema:"description=If the source is an archive extract the specified file from the archive"`
	Symlinks   []string `json:"symlinks,omitempty" jsonschema:"description=List of symlinks to create during package deploy"`
}

type ZarfFileHashList struct {
	Source    string `json:"source" jsonschema:"description=Local file path or remote URL of the hash file"`
	Algorithm string `json:"algorithm" jsonschema:"description=The algorithm used to generate the hash file,enum=MD5,SHA1,SHA224,SHA256,SHA384,SHA512"`
	Target    string `json:"target" jsonschema:"description=The absolute or relative path where the collected files should be copied to during package deploy"`
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	Name        string   `json:"name" jsonschema:"description=The name of the chart to deploy; this should be the name of the chart as it is installed in the helm repo"`
	ReleaseName string   `json:"releaseName,omitempty" jsonschema:"description=The name of the release to create; defaults to the name of the chart"`
	Url         string   `json:"url,omitempty" jsonschema:"oneof_required=url,description=The URL of the chart repository or git url if the chart is using a git repo instead of helm repo"`
	Version     string   `json:"version" jsonschema:"description=The version of the chart to deploy; for git-based charts this is also the tag of the git repo"`
	Namespace   string   `json:"namespace" jsonschema:"description=The namespace to deploy the chart to"`
	ValuesFiles []string `json:"valuesFiles,omitempty" jsonschema:"description=List of values files to include in the package; these will be merged together"`
	GitPath     string   `json:"gitPath,omitempty" jsonschema:"description=The path to the chart in the repo if using a git repo instead of a helm repo"`
	LocalPath   string   `json:"localPath,omitempty" jsonschema:"oneof_required=localPath,description=The path to the chart folder"`
	NoWait      bool     `json:"noWait,omitempty" jsonschema:"description=Wait for chart resources to be ready before continuing"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart
type ZarfManifest struct {
	Name                       string   `json:"name" jsonschema:"description=A name to give this collection of manifests; this will become the name of the dynamically-created helm chart"`
	Namespace                  string   `json:"namespace,omitempty" jsonschema:"description=The namespace to deploy the manifests to"`
	Files                      []string `json:"files,omitempty" jsonschema:"description=List of individual K8s YAML files to deploy (in order)"`
	KustomizeAllowAnyDirectory bool     `json:"kustomizeAllowAnyDirectory,omitempty" jsonschema:"description=Allow traversing directory above the current directory if needed for kustomization"`
	Kustomizations             []string `json:"kustomizations,omitempty" jsonschema:"description=List of kustomization paths to include in the package"`
	NoWait                     bool     `json:"noWait,omitempty" jsonschema:"description=Wait for manifest resources to be ready before continuing"`
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

type ZarfComponentActions struct {
	Create ZarfComponentActionSet `json:"create,omitempty" jsonschema:"description=Actions to run during package creation"`
	Deploy ZarfComponentActionSet `json:"deploy,omitempty" jsonschema:"description=Actions to run during package deployment"`
	Remove ZarfComponentActionSet `json:"remove,omitempty" jsonschema:"description=Actions to run during package removal"`
}

type ZarfComponentActionSet struct {
	First   []ZarfComponentAction `json:"first,omitempty" jsonschema:"description=Actions to run at the start of an operation"`
	Last    []ZarfComponentAction `json:"last,omitempty" jsonschema:"description=Actions to run at the end of an operation"`
	Success []ZarfComponentAction `json:"success,omitempty" jsonschema:"description=Actions to run if all operations succeed"`
	Failure []ZarfComponentAction `json:"failure,omitempty" jsonschema:"description=Actions to run if all operations fail"`
}

type ZarfComponentAction struct {
	Mute       bool     `json:"mute,omitempty" jsonschema:"description=Hide the output of the script during package deployment"`
	MaxSeconds int      `json:"maxSeconds,omitempty" jsonschema:"description=Timeout in seconds for the script"`
	Retry      bool     `json:"retry,omitempty" jsonschema:"description=Retry the script if it fails"`
	Env        []string `json:"env,omitempty" jsonschema:"description=Environment variables to set for the script"`
	Cmd        string   `json:"cmd,omitempty" jsonschema:"description=The script to run"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
	Path      string `json:"path" jsonschema:"description=The path to copy the data to in the container"`
}

// ZarfDataInjection is a data-injection definition
type ZarfDataInjection struct {
	Source   string              `json:"source" jsonschema:"description=A path to a local folder or file to inject into the given target pod + container"`
	Target   ZarfContainerTarget `json:"target" jsonschema:"description=The target pod + container to inject the data into"`
	Compress bool                `json:"compress,omitempty" jsonschema:"description=Compress the data before transmitting using gzip.  Note: this requires support for tar/gzip locally and in the target image."`
}

// ZarfImport structure for including imported zarf components
type ZarfComponentImport struct {
	ComponentName string `json:"name,omitempty"`
	// For further explanation see https://regex101.com/library/Ldx8yG and https://regex101.com/r/Ldx8yG/1
	Path string `json:"path" jsonschema:"pattern=^(?!.*###ZARF_PKG_VAR_).*$"`
}
