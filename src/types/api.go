package types

type RestAPI struct {
	ZarfPackage       ZarfPackage       `json:"zarfPackage"`
	ZarfState         ZarfState         `json:"zarfState"`
	ZarfCommonOptions ZarfCommonOptions `json:"zarfCommonOptions"`
	ZarfCreateOptions ZarfCreateOptions `json:"zarfCreateOptions"`
	ZarfDeployOptions ZarfDeployOptions `json:"zarfDeployOptions"`
	ConnectStrings    ConnectStrings    `json:"connectStrings"`
	ClusterSummary    ClusterSummary    `json:"clusterSummary"`
	DeployedPackage   DeployedPackage   `json:"deployedPackage"`
	APIZarfPackage    APIZarfPackage    `json:"apiZarfPackage"`
}

type ClusterSummary struct {
	Reachable bool      `json:"reachable"`
	HasZarf   bool      `json:"hasZarf"`
	Distro    string    `json:"distro"`
	ZarfState ZarfState `json:"zarfState"`
}

type APIZarfPackage struct {
	Path        string      `json:"path"`
	ZarfPackage ZarfPackage `json:"zarfPackage"`
}
