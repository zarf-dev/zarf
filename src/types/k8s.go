package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance" jsonschema:"description=Indicates if Zarf was initialized while deploying its own k8s cluster"`
	Distro        string       `json:"distro" jsonschema:"description=K8s distribution of the cluster Zarf was deployed to"`
	Architecture  string       `json:"architecture" jsonschema:"description=Machine architecture of the k8s node(s)"`
	StorageClass  string       `json:"storageClass" jsonschema:"StorageClass of the k8s cluster Zarf was deployed to"`
	Secret        string       `json:"secret"`
	NodePort      string       `json:"nodePort"`
	AgentTLS      GeneratedPKI `json:"agentTLS" jsonschema:"PKI certificate information for the agent pods Zarf manages"`

	InstalledPackages map[string]InstalledPackage `json:"installedPackages" jsonschema:"description=Map of all the things Zarf has installed so far.."`
}

type InstalledPackage struct {
	// InstalledCharts     map[string]InstalledComponent // key={package_name}
	PackageVersion string
	CLIVersion     string
	// InstalledCharts []string
	InstalledComponents map[string]InstalledComponent
}

type InstalledComponent struct {
	InstalledCharts []InstalledCharts
}

type InstalledCharts struct {
	Namespace string
	ChartName string
}

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}
