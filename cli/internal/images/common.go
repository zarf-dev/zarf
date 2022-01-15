package images

import (
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

var cachePath = ".zarf-image-cache"

var cranePlatformOptions = crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"})

func init() {
	homePath, _ := os.UserHomeDir()
	cachePath = fmt.Sprintf("%s/%s", homePath, cachePath)
}
