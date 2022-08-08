package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance"`
	Distro        string       `json:"distro"`
	Architecture  string       `json:"architecture"`
	StorageClass  string       `json:"storageClass"`
	AgentTLS      GeneratedPKI `json:"agentTLS"`

	GitServer             GitServerInfo         `json:"gitServer"`
	ContainerRegistryInfo ContainerRegistryInfo `json:"containerRegistryInfo"`
	LoggingPassword       string                `json:"loggingPassword"`
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
	CA   []byte
	Cert []byte
	Key  []byte
}
