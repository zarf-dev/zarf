package images

import "github.com/defenseunicorns/zarf/src/types"

type ImgConfig struct {
	TarballPath string

	ImgList []string

	RegInfo types.RegistryInfo

	NoChecksum bool

	Insecure bool
}

func New(config *ImgConfig) *ImgConfig {
	return config
}
