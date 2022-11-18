package types

type RestAPI struct {
	ZarfPackage       ZarfPackage       `json:"zarfPackage"`
	ZarfState         ZarfState         `json:"zarfState"`
	ZarfCommonOptions ZarfCommonOptions `json:"zarfCommonOptions"`
	ZarfCreateOptions ZarfCreateOptions `json:"zarfCreateOptions"`
	ZarfDeployOptions ZarfDeployOptions `json:"zarfDeployOptions"`
	ZarfInitOptions   ZarfInitOptions   `json:"zarfInitOptions"`
	ConnectStrings    ConnectStrings    `json:"connectStrings"`
	ClusterSummary    ClusterSummary    `json:"clusterSummary"`
	DeployedPackage   DeployedPackage   `json:"deployedPackage"`
	APIZarfPackage    APIZarfPackage    `json:"apiZarfPackage"`
	APIError          APIError          `json:"error"`
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

// TODO: decide as a team how JSON errors should be formatted
type APIError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
