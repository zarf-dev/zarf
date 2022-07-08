package types

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	Confirm       bool
	TempDirectory string
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment
type ZarfDeployOptions struct {
	PackagePath string
	Components  string
	SGetKeyPath string

	// Zarf init is installing the k3s component
	ApplianceMode bool

	// Zarf init override options
	StorageClass string
	Secret       string
	NodePort     string
}

// ZarfCreeateOptions tracks the user-defined options used to create the package
type ZarfCreateOptions struct {
	SkipSBOM        bool
	ImageCachePath  string
	Insecure        bool
	OutputDirectory string
}

type ConnectString struct {
	Description string
	Url         string
}
type ConnectStrings map[string]ConnectString
