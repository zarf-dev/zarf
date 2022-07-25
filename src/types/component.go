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
	Source     string   `yaml:"source"`
	Shasum     string   `yaml:"shasum,omitempty"`
	Target     string   `yaml:"target"`
	Executable bool     `yaml:"executable,omitempty"`
	Symlinks   []string `yaml:"symlinks,omitempty"`
}

// ZarfChart defines a helm chart to be deployed.
type ZarfChart struct {
	Name        string   `yaml:"name"`
	ReleaseName string   `yaml:"releaseName,omitempty"`
	Url         string   `yaml:"url"`
	Version     string   `yaml:"version"`
	Namespace   string   `yaml:"namespace"`
	ValuesFiles []string `yaml:"valuesFiles,omitempty"`
	GitPath     string   `yaml:"gitPath,omitempty"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart
type ZarfManifest struct {
	Name                       string   `yaml:"name"`
	DefaultNamespace           string   `yaml:"namespace,omitempty"`
	Files                      []string `yaml:"files,omitempty"`
	KustomizeAllowAnyDirectory bool     `yaml:"kustomizeAllowAnyDirectory,omitempty"`
	Kustomizations             []string `yaml:"kustomizations,omitempty"`
}

// ZarfComponentScripts are scripts that run before or after a component is deployed
type ZarfComponentScripts struct {
	ShowOutput     bool     `yaml:"showOutput,omitempty"`
	TimeoutSeconds int      `yaml:"timeoutSeconds,omitempty"`
	Retry          bool     `yaml:"retry,omitempty"`
	Before         []string `yaml:"before,omitempty"`
	After          []string `yaml:"after,omitempty"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	Namespace string `yaml:"namespace"`
	Selector  string `yaml:"selector"`
	Container string `yaml:"container,omitempty"`
	Path      string `yaml:"path"`
}

// ZarfDataInjection is a data-injection definition
type ZarfDataInjection struct {
	Source string              `yaml:"source"`
	Target ZarfContainerTarget `yaml:"target"`
}

// ZarfImport structure for including imported zarf components
type ZarfComponentImport struct {
	ComponentName string `yaml:"name,omitempty"`
	Path          string `yaml:"path" jsonschema:"pattern=^(?!.*###ZARF_PKG_VAR_).*$"`
}
