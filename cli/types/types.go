package types

// ZarfFile defines a file to deploy
type ZarfFile struct {
	Source     string   `yaml:"source"`
	Shasum     string   `yaml:"shasum,omitempty"`
	Target     string   `yaml:"target"`
	Executable bool     `yaml:"executable,omitempty"`
	Symlinks   []string `yaml:"symlinks,omitempty"`
}

// ZarfChart defines a helm chart to be deployed
type ZarfChart struct {
	Name        string   `yaml:"name"`
	Url         string   `yaml:"url"`
	Version     string   `yaml:"version"`
	Namespace   string   `yaml:"namespace"`
	ValuesFiles []string `yaml:"valuesFiles,omitempty"`
	GitPath     string   `yaml:"gitPath,omitempty"`
}

// ZarfComponent is the primary functional grouping of assets to deploy by zarf
type ZarfComponent struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description,omitempty"`
	Default     bool                 `yaml:"default,omitempty"`
	Required    bool                 `yaml:"required,omitempty"`
	Files       []ZarfFile           `yaml:"files,omitempty"`
	Charts      []ZarfChart          `yaml:"charts,omitempty"`
	Manifests   []ZarfManifest       `yaml:"manifests,omitempty"`
	Images      []string             `yaml:"images,omitempty"`
	Repos       []string             `yaml:"repos,omitempty"`
	Scripts     ZarfComponentScripts `yaml:"scripts,omitempty"`
	Connect     []ZarfConnect        `yaml:"connect,omitempty"`
}

// ZarfConnect defines tunnel parameters a component can use with zarf connect to expose a service or pod
type ZarfConnect struct {
	Identifier string `yaml:"identifier"`
	Namespace  string `yaml:"namespace"`
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	RemotePort int    `yaml:"remotePort"`
	LocalPort  int    `yaml:"localPort,omitempty"`
}

// ZarfManifest defines raw manifests Zarf will deploy as a helm chart
type ZarfManifest struct {
	Name             string   `yaml:"name"`
	DefaultNamespace string   `yaml:"namespace,omitempty"`
	Files            []string `yaml:"files"`
}

// ZarfComponentScripts are scripts that run before or after a component is deployed
type ZarfComponentScripts struct {
	Retry  bool     `yaml:"retry,omitempty"`
	Before []string `yaml:"before,omitempty"`
	After  []string `yaml:"after,omitempty"`
}

// ZarfMetadata lists information about the current ZarfPackage
type ZarfMetadata struct {
	Name         string `yaml:"name,omitempty"`
	Description  string `yaml:"description,omitempty"`
	Version      string `yaml:"version,omitempty"`
	Url          string `yaml:"url,omitempty"`
	Image        string `yaml:"image,omitempty"`
	Uncompressed bool   `yaml:"uncompressed,omitempty"`
}

// ZarfContainerTarget defines the destination info for a ZarfData target
type ZarfContainerTarget struct {
	Namespace string `yaml:"namespace"`
	Selector  string `yaml:"selector"`
	Container string `yaml:"container,omitempty"`
	Path      string `yaml:"path"`
}

// ZarfData is a data-injection definition
type ZarfData struct {
	Source string              `yaml:"source"`
	Target ZarfContainerTarget `yaml:"target"`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package
type ZarfBuildData struct {
	Terminal  string `yaml:"terminal"`
	User      string `yaml:"user"`
	Arch      string `yaml:"arch"`
	Timestamp string `yaml:"timestamp"`
	Version   string `yaml:"string"`
}

// ZarfPackage the top-level structure of a Zarf config file
type ZarfPackage struct {
	Kind       string          `yaml:"kind,omitempty"`
	Metadata   ZarfMetadata    `yaml:"metadata,omitempty"`
	Build      ZarfBuildData   `yaml:"build,omitempty"`
	Data       []ZarfData      `yaml:"data,omitempty"`
	Components []ZarfComponent `yaml:"components,omitempty"`
	Seed       []string        `yaml:"seed,omitempty"`
}

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool   `json:"zarfAppliance"`
	Distro        string `json:"distro"`
	StorageClass  string `json:"storageClass"`
	Secret        string `json:"secret"`
	Registry      struct {
		SeedType string `json:"seedType"`
		NodePort string `json:"nodePort"`
	} `json:"registry"`
}

// TLSConfig tracks the user-defined options for TLS cert generation
type TLSConfig struct {
	CertPublicPath  string `yaml:"certPublicPath"`
	CertPrivatePath string `yaml:"certPrivatePath"`
	Host            string `yaml:"host"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment
type ZarfDeployOptions struct {
	PackagePath   string
	Confirm       bool
	Components    string
	ApplianceMode bool
}
