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

//TODO: Should the password for the GitServerINfo be a secret/encoded?
type GitServerInfo struct {
	GitAddress  string `json:"gitAddress"`
	GitUsername string `json:"gitUsername"`
	GitPassword string `json:"gitPassword"`
	GitPort     int    `json:"gitPort"`
}

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}
