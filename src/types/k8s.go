package types

// ZarfState is maintained as a secret in the Zarf namespace to track Zarf init data
type ZarfState struct {
	ZarfAppliance bool         `json:"zarfAppliance"`
	Distro        string       `json:"distro"`
	Architecture  string       `json:"architecture"`
	StorageClass  string       `json:"storageClass"`
	Secret        string       `json:"secret"`
	NodePort      string       `json:"nodePort"`
	AgentTLS      GeneratedPKI `json:"agentTLS"`

	GitServerInfo GitServerInfo `json:"gitServerInfo"`
}

type GitServerInfo struct {
	GitAddress      string `json:"gitAddress"`
	GitPushUsername string `json:"gitPushUsername"`
	GitPushPassword string `json:"gitPushPassword"`
	GitReadUsername string `json:"gitReadUsername"`
	GitReadPassword string `json:"gitReadPassword"`
	GitPort         int    `json:"gitPort"`
	InternalServer  bool   `json:"internalServer"`
}

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}
