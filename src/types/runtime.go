package types

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	Confirm       bool   `json:"confirm"`
	TempDirectory string `json:"tempDirectory"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment
type ZarfDeployOptions struct {
	PackagePath string `json:"packagePath"`
	Components  string `json:"components"`
	SGetKeyPath string `json:"sGetKeyPath"`

	// Zarf init is installing the k3s component
	ApplianceMode bool `json:"applianceMode"`

	// Zarf init override options
	StorageClass string `json:"storageClass"`
	Secret       string `json:"secret"`
	NodePort     string `json:"nodePort"`

	SetVariables map[string]string `json:"setVariables"`
}

// ZarfCreateOptions tracks the user-defined options used to create the package.
type ZarfCreateOptions struct {
	SkipSBOM        bool              `json:"skipSBOM"`
	ImageCachePath  string            `json:"imageCachePath"`
	Insecure        bool              `json:"insecure"`
	OutputDirectory string            `json:"outputDirectory"`
	SetVariables    map[string]string `json:"setVariables"`
}

type ConnectString struct {
	Description string `json:"description"`
	Url         string `json:"url"`
}
type ConnectStrings map[string]ConnectString
