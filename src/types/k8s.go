package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string       `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string       `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string       `json:"storageClass" jsonschema:"StorageClass of the k8s cluster Zarf was deployed to"`
	AgentTLS      GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	GitServer     GitServerInfo `json:"gitServer" jsonschema:"description=Information about the repository Zarf is configured to use"`
	RegistryInfo  RegistryInfo  `json:"registryInfo" jsonschema:"description=Information about the registry Zarf is configured to use"`
	LoggingSecret string        `json:"loggingSecret" jsonschema:"description=Secret value that the internal Grafana server was seeded with"`

	InstalledPackages map[string]InstalledPackage `json:"installedPackages" jsonschema:"description=Map of all the things Zarf has installed so far.."`
}

type InstalledPackage struct {
	// InstalledCharts     map[string]InstalledComponent // key={package_name}
	PackageVersion string
	PackageYaml    string
	CLIVersion     string

	InstalledComponents map[string]InstalledComponent
}

type InstalledComponent struct {
	InstalledCharts []InstalledCharts
}

type InstalledCharts struct {
	Namespace string
	ChartName string
}

type GitServerInfo struct {
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the git repository"`
	PushPassword string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the git repository"`
	ReadUsername string `json:"readUsername" jsonschema:"description=Username of a user with read-only access to the git repository. If not provided for an external repository than the push-user is used"`
	ReadPassword string `json:"readPassword" jsonschema:"description=Password of a user with read-only access to the git repository. If not provided for an external repository than the push-user is used"`

	Address        string `json:"address" jsonschema:"description=URL address of the git server"`
	InternalServer bool   `json:"internalServer" jsonschema:"description=Indicates if we are using a git server that Zarf is directly managing"`
}

type RegistryInfo struct {
	PushUsername string `json:"pushUsername" jsonschema:"description=Username of a user with push access to the registry"`
	PushPassword string `json:"pushPassword" jsonschema:"description=Password of a user with push access to the registry"`
	PullUsername string `json:"pullUsername" jsonschema:"description=Username of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used"`
	PullPassword string `json:"pullPassword" jsonschema:"description=Password of a user with pull-only access to the registry. If not provided for an external registry than the push-user is used"`

	Address          string `json:"address" jsonschema:"description=URL address of the registry"`
	NodePort         int    `json:"nodePort" jsonschema:"description=Nodeport of the registry. Only needed if the registry is running inside the kubernetes cluster"`
	InternalRegistry bool   `json:"internalRegistry" jsonschema:"description=Indicates if we are using a registry that Zarf is directly managing"`

	Secret string `json:"secret" jsonschema:"description=Secret value that the registry was seeded with"`
}

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}
