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

	GitServer GitServerInfo `json:"gitServerInfo"`
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

type GeneratedPKI struct {
	CA   []byte
	Cert []byte
	Key  []byte
}
