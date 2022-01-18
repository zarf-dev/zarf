package images

import (
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var cachePath = ".zarf-image-cache"

var cranePlatformAMD64 = crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})
var cranePlatformARM64 = crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "arm64"})

func init() {
	homePath, _ := os.UserHomeDir()
	cachePath = fmt.Sprintf("%s/%s", homePath, cachePath)
}
