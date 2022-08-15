package types

// ZarfCommonOptions tracks the user-defined preferences used across commands.
type ZarfCommonOptions struct {
	Confirm       bool              `json:"confirm" jsonschema:"description=Verify that Zarf should perform an action"`
	TempDirectory string            `json:"tempDirectory" jsonschema:"description=Location Zarf should use as a staging ground when managing files and images for package creation and deployment"`
	SetVariables  map[string]string `json:"setVariables" jsonschema:"description=Key-Value map of variable names and their corresponding values that will be used to template against the Zarf package being used"`
}

// ZarfDeployOptions tracks the user-defined preferences during a package deployment
type ZarfDeployOptions struct {
	PackagePath string `json:"packagePath" jsonschema:"description=Location where a Zarf package to deploy can be found"`
	Components  string `json:"components" jsonschema:"description=Comma separated list of optional components to deploy"`
	SGetKeyPath string `json:"sGetKeyPath" jsonschema:"description=Location where the public key component of a cosign key-pair can be found"`

	// Zarf init is installing the k3s component
	ApplianceMode bool `json:"applianceMode" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
}

// Zarf InitOptions tracks the user-defined options during cluster initialization
type ZarfInitOptions struct {
	// Misc init overrides..
	StorageClass string `json:"storageClass" jsonschema:"description=StorageClass of the k8s cluster Zarf is initializing"`

	// Using a remote git server
	GitServer GitServerInfo `json:"gitServer" jsonschema:"description=Information about the repository Zarf is going to be using"`

	RegistryInfo RegistryInfo `json:"registryInfo" jsonschema:"description=Information about the registry Zarf is going to be using"`
}

// ZarfCreateOptions tracks the user-defined options used to create the package
type ZarfCreateOptions struct {
	SkipSBOM        bool   `json:"skipSBOM" jsonschema:"description=Disable the generation of SBOM materials during package creation"`
	ImageCachePath  string `json:"imageCachePath" jsonschema:"description=Path to where a .cache directory of cached image that were pulled down to create packages"`
	Insecure        bool   `json:"insecure" jsonschema:"description=Disable the need for shasum validations when pulling down files from the internet"`
	OutputDirectory string `json:"outputDirectory" jsonschema:"description=Location where the finalized Zarf package will be placed"`
}

type ConnectString struct {
	Description string `json:"description" jsonschema:"description=Descriptive text that explains what the resource you would be connecting to is used for"`
	Url         string `json:"url" jsonschema:"description=URL path that gets appended to the k8s port-forward result"`
}
type ConnectStrings map[string]ConnectString
