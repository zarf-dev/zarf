package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string       `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string       `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string       `json:"storageClass" jsonschema:"Default StorageClass value Zarf uses for variable templating"`
	AgentTLS      GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	GitServer     GitServerInfo `json:"gitServer"`
	RegistryInfo  RegistryInfo  `json:"registryInfo"`
	LoggingSecret string        `json:"loggingSecret"`
}

type DeployedPackage struct {
	Name       string      `json:"name"`
	Data       ZarfPackage `json:"data"`
	CLIVersion string      `json:"cliVersion"`

	DeployedComponents []DeployedComponent `json:"deployedComponents"`
}

type DeployedComponent struct {
	Name            string            `json:"name"`
	InstalledCharts []InstalledCharts `json:"installedCharts"`
}

type InstalledCharts struct {
	Namespace string `json:"namespace"`
	ChartName string `json:"chartName"`
}

// TODO: Should the password for the GitServerINfo be a secret/encoded?
type GitServerInfo struct {
	PushUsername string `json:"pushUsername"`
	PushPassword string `json:"pushPassword"`
	ReadUsername string `json:"readUsername"`
	ReadPassword string `json:"readPassword"`

	Address        string `json:"address"`
	Port           int    `json:"port"`
	InternalServer bool   `json:"internalServer"`
}

type RegistryInfo struct {
	PushUsername string `json:"pushUsername"`
	PushPassword string `json:"pushPassword"`
	PullUsername string `json:"pullUsername"`
	PullPassword string `json:"pullPassword"`

	Address          string `json:"address"`
	NodePort         int    `json:"nodePort"`
	InternalRegistry bool   `json:"internalRegistry"`

	Secret string `json:"secret"`
}

type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
