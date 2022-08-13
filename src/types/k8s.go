package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance"`
	Distro        string       `json:"distro"`
	Architecture  string       `json:"architecture"`
	StorageClass  string       `json:"storageClass"`
	AgentTLS      GeneratedPKI `json:"agentTLS"`

	GitServer     GitServerInfo `json:"gitServer"`
	RegistryInfo  RegistryInfo  `json:"registryInfo"`
	LoggingSecret string        `json:"loggingSecret"`
}

type GitServerInfo struct {
	PushUsername string `json:"pushUsername"`
	PushPassword string `json:"pushPassword"`
	ReadUsername string `json:"readUsername"`
	ReadPassword string `json:"readPassword"`

	Address string `json:"address"`
	Port    int    `json:"port"`

	InternalServer bool `json:"internalServer"`
}

type RegistryInfo struct {
	PushUsername string `json:"pushUsername"`
	PushPassword string `json:"pushPassword"`
	PullUsername string `json:"pullUsername"`
	PullPassword string `json:"pullPassword"`

	Address  string `json:"address"`
	NodePort int    `json:"nodePort"`

	InternalRegistry bool `json:"internalRegistry"`
}

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}
