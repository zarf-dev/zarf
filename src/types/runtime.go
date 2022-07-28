package types

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	Confirm       bool              `json:"confirm"`
	TempDirectory string            `json:"tempDirectory"`
	SetVariables  map[string]string `json:"setVariables"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment
type ZarfDeployOptions struct {
	PackagePath string `json:"packagePath"`
	Components  string `json:"components"`
	SGetKeyPath string `json:"sGetKeyPath"`

	GitServer GitServerInfo
}

// Zarf InitOptions tracks the user-defined options during cluster initialization
type ZarfInitOptions struct {
	// Misc init overrides..
	ApplianceMode bool   `json:"applianceMode"`
	StorageClass  string `json:"storageClass"`
	Secret        string `json:"secret"`
	NodePort      string `json:"nodePort"`

	// Using a remote git server
	GitServer GitServerInfo

	ContainerRegistryInfo ContainerRegistryInfo
}

// ZarfCreateOptions tracks the user-defined options used to create the package
type ZarfCreateOptions struct {
	SkipSBOM        bool   `json:"skipSBOM"`
	ImageCachePath  string `json:"imageCachePath"`
	Insecure        bool   `json:"insecure"`
	OutputDirectory string `json:"outputDirectory"`
}

type ConnectString struct {
	Description string `json:"description"`
	Url         string `json:"url"`
}
type ConnectStrings map[string]ConnectString
