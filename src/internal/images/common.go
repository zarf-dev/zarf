package images

import (
	"fmt"
	"os"
)

var cachePath = ".zarf-image-cache"

func init() {
	homePath, _ := os.UserHomeDir()
	cachePath = fmt.Sprintf("%s/%s", homePath, cachePath)
}

func CachePath() string {
	return cachePath
}
