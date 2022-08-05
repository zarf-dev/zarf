package types

// ZarfComponent is the primary functional grouping of assets to deploy by zarf.
type ZarfComponent struct {
	// Name is the unique identifier for this component
	Name string `yaml:"name" jsonschema:"description=The name of the component,pattern=^[a-z0-9\\-]+$"`

	// Description is a message given to a user when deciding to enable this componenent or not
	Description string `yaml:"description,omitempty" jsonschema:"description=Message to include during package deploy describing the purpose of this component"`

	// Default changes the default option when deploying this component
	Default bool `yaml:"default,omitempty" jsonschema:"description=Determines the default Y/N state for installing this component on package deploy"`

	// Required makes this component mandatory for package deployment
	Required bool `yaml:"required,omitempty" jsonschema:"description=Do not prompt user to install this component, always install on package deploy"`

	// Only include compatible components during package deployment
	Only ZarfComponentOnlyTarget `yaml:"only,omitempty" jsonschema:"description=Filter when this component is included in package creation or deployment"`

	// Key to match other components to produce a user selector field, used to create a BOOLEAN XOR for a set of components
	// Note: ignores default and required flags
	Group string `yaml:"group,omitempty" jsonschema:"description=Create a user selector field based on all components in the same group"`

	//Path to cosign publickey for signed online resources
	CosignKeyPath string `yaml:"cosignKeyPath,omitempty" jsonschema:"description=Specify a path to a public key to validate signed online resources"`

	// Import refers to another zarf.yaml package component.
	Import ZarfComponentImport `yaml:"import,omitempty" jsonschema:"description=Import a component from another Zarf package"`

	// Scripts are custom commands that run before or after package deployment
	Scripts ZarfComponentScripts `yaml:"scripts,omitempty" jsonschema:"description=Custom commands to run before or after package deployment"`

	// Files are files to place on disk during deploy
	Files []ZarfFile `yaml:"files,omitempty" jsonschema:"description=Files to place on disk during package deployment"`

	// Charts are helm charts to install during package deploy
	Charts []ZarfChart `yaml:"charts,omitempty" jsonschema:"description=Helm charts to install during package deploy"`

	// Manifests are raw manifests that get converted into zarf-generated helm charts during deploy
	Manifests []ZarfManifest `yaml:"manifests,omitempty"`

	// Images are the online images needed to be included in the zarf package
	Images []string `yaml:"images,omitempty" jsonschema:"description=List of OCI images to include in the package"`

	// Repos are any git repos that need to be pushed into the git server
	Repos []string `yaml:"repos,omitempty" jsonschema:"description=List of git repos to include in the package"`

	// Data pacakges to push into a running cluster
	DataInjections []ZarfDataInjection `yaml:"dataInjections,omitempty" jsonschema:"description=Datasets to inject into a pod in the target cluster"`
}

// ZarfComponentOnlyTarget filters a component to only show it for a given OS/Arch
type ZarfComponentOnlyTarget struct {
	LocalOS string                   `yaml:"localOS,omitempty" jsonschema:"description=Only deploy component to specified OS,enum=linux,enum=darwin,enum=windows"`
	Cluster ZarfComponentOnlyCluster `yaml:"cluster,omitempty" jsonschema:"description=Only deploy component to specified clusters"`
}

type ZarfComponentOnlyCluster struct {
	Architecture string   `yaml:"architecture,omitempty" jsonschema:"description=Only create and deploy to clusters of the given architecture,enum=amd64,enum=arm64"`
	Distros      []string `yaml:"distros,omitempty" jsonschema:"description=Future use"`
}

// ZarfFile defines a file to deploy.
type ZarfFile struct {
	Source     string   `yaml:"source" jsonschema:"description=Local file path or remote URL to add to the package"`
	Shasum     string   `yaml:"shasum,omitempty" jsonschema:"description=SHA256 checksum of the file if the source is a URL"`
	Target     string   `yaml:"target" json:"target" jsonschema:"description=The absolute or relative path wher the file should be copied to during package deploy"`
	Executable bool     `yaml:"executable,omitempty" jsonschema:"description=Determines if the file should be made executable during package deploy"`
	Symlinks   []string `yaml:"symlinks,omitempty" jsonschema:"description=List of symlinks to create during package deploy"`
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	Name        string   `yaml:"name" jsonschema:"description=The name of the chart to deploy, this should be the name of the chart as it is installed in the helm repo"`
	ReleaseName string   `yaml:"releaseName,omitempty" jsonschema:"description=The name of the release to create, defaults to the name of the chart"`
	Url         string   `yaml:"url" jsonschema:"description=The URL of the chart repository or git url if the chart is using a git repo instead of helm repo"`
	Version     string   `yaml:"version" jsonschema:"description=The version of the chart to deploy, for git-based charts this is also the tag of the git repo"`
	Namespace   string   `yaml:"namespace" jsonschema:"description=The namespace to deploy the chart to"`
	ValuesFiles []string `yaml:"valuesFiles,omitempty" jsonschema:"description=List of values files to include in the package, these will be merged together"`
	GitPath     string   `yaml:"gitPath,omitempty" jsonschema:"description=If using a git repo, the path to the chart in the repo"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart
type ZarfManifest struct {
	Name                       string   `yaml:"name" jsonschema:"description=A name to give this collection of manifests, this will become the name of the dynamically-created helm chart"`
	DefaultNamespace           string   `yaml:"namespace,omitempty" jsonschema:"description=The namespace to deploy the manifests to"`
	Files                      []string `yaml:"files,omitempty" jsonschema:"description=List of individual K8s YAML files to deploy (in order)"`
	KustomizeAllowAnyDirectory bool     `yaml:"kustomizeAllowAnyDirectory,omitempty" jsonschema:"description=Allow traversing directory above the current directory if needed for kustomization"`
	Kustomizations             []string `yaml:"kustomizations,omitempty" jsonschema:"description=List of kustomization paths to include in the package"`
}

// ZarfComponentScripts are scripts that run before or after a component is deployed
type ZarfComponentScripts struct {
	ShowOutput     bool                       `yaml:"showOutput,omitempty" jsonschema:"description=Show the output of the script during package deployment"`
	TimeoutSeconds int                        `yaml:"timeoutSeconds,omitempty" jsonschema:"description=Timeout in seconds for the script"`
	Retry          bool                       `yaml:"retry,omitempty" jsonschema:"description=Retry the script if it fails"`
	Before         []string                   `yaml:"before,omitempty" jsonschema:"description=Scripts to run before the component is deployed"`
	After          []string                   `yaml:"after,omitempty" jsonschema:"description=Scripts to run after the component successfully deploys"`
	Create         ZarfComponentCreateScripts `yaml:"create,omitempty" jsonschema:"description=Scripts to run during package creation"`
}

// ZarfComponentCreateScripts are scripts that run during package creation
type ZarfComponentCreateScripts struct {
	Before []string `yaml:"before,omitempty" jsonschema:"description=Scripts to run after the component is added during package creation"`
	After  []string `yaml:"after,omitempty" jsonschema:"description=Scripts to run before the component is added during package creation"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	Namespace string `yaml:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `yaml:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `yaml:"container" jsonschema:"description=The container to target for data injection"`
	Path      string `yaml:"path" jsonschema:"description=The path to copy the data to in the container"`
}

// ZarfDataInjection is a data-injection definition
type ZarfDataInjection struct {
	Source   string              `yaml:"source" jsonschema:"description=A path to a local folder or file to inject into the given target pod + container"`
	Target   ZarfContainerTarget `yaml:"target" jsonschema:"description=The target pod + container to inject the data into"`
	Compress bool                `yaml:"compress,omitempty" jsonschema:"description=Compress the data before transmitting using gzip.  Note: this requires support for tar/gzip locally and in the target image."`
}

// ZarfImport structure for including imported zarf components
type ZarfComponentImport struct {
	ComponentName string `yaml:"name,omitempty"`
	// For further explanation see https://regex101.com/library/Ldx8yG and https://regex101.com/r/Ldx8yG/1
	Path string `yaml:"path" jsonschema:"pattern=^(?!.*###ZARF_PKG_VAR_).*$"`
}
