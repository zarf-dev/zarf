package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string       `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string       `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string       `json:"storageClass" jsonschema:"Default StorageClass value Zarf uses for variable templating"`
	Secret        string       `json:"secret"`
	AgentTLS      GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	GitServer             GitServerInfo         `json:"gitServer"`
	ContainerRegistryInfo ContainerRegistryInfo `json:"containerRegistryInfo"`
	LoggingPassword       string                `json:"loggingPassword"`
}

type DeployedPackage struct {
	Name               string                       `json:"name"`
	Data               ZarfPackage                  `json:"data"`
	CLIVersion         string                       `json:"cliVersion"`
	DeployedComponents map[string]DeployedComponent `json:"deployedComponents"`
}

type DeployedComponent struct {
	InstalledCharts []InstalledCharts `json:"installedCharts"`
}

type InstalledCharts struct {
	Namespace string `json:"namespace"`
	ChartName string `json:"chartName"`
}

type GitServerInfo struct {
	Address        string `json:"gitAddress"`
	PushUsername   string `json:"gitPushUsername"`
	PushPassword   string `json:"gitPushPassword"`
	ReadUsername   string `json:"gitReadUsername"`
	ReadPassword   string `json:"gitReadPassword"`
	Port           int    `json:"gitPort"`
	InternalServer bool   `json:"internalServer"`
}

type ContainerRegistryInfo struct {
	PushUser     string `json:"pushUser"`
	PushPassword string `json:"pushPassword"`

	PullUser     string `json:"pullUser"`
	PullPassword string `json:"pullPassword"`

	URL string `json:"URL"`

	InternalRegistry bool `json:"internalRegistry"`

	NodePort int `json:"nodePort"`
}

type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
