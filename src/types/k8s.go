package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string       `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string       `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string       `json:"storageClass" jsonschema:"Default StorageClass value Zarf uses for variable templating"`
	Secret        string       `json:"secret"`
	AgentTLS      GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	GitServer GitServerInfo `json:"gitServer"`

	ContainerRegistryInfo ContainerRegistryInfo `json:"containerRegistryInfo"`
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
	Address        string `json:"gitAddress"`
	PushUsername   string `json:"gitPushUsername"`
	PushPassword   string `json:"gitPushPassword"`
	ReadUsername   string `json:"gitReadUsername"`
	ReadPassword   string `json:"gitReadPassword"`
	Port           int    `json:"gitPort"`
	InternalServer bool   `json:"internalServer"`
}

type ContainerRegistryInfo struct {
	RegistryPushUser     string `json:"registryPushUser"`
	RegistryPushPassword string `json:"registryPushPassword"`

	RegistryPullUser     string `json:"registryPullUser"`
	RegistryPullPassword string `json:"registryPullPassword"`

	RegistrySecret string `json:"registrySecret"` // TODO: @JPERRY figure out what this is doing..

	RegistryURL string `json:"registryURL"`

	InternalRegistry bool `json:"internalRegistry"`

	NodePort int `json:"nodePort"` // TODO @JPERRY: Figure out the difference between this port and the one provided at the end of svc URL
}

type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}
