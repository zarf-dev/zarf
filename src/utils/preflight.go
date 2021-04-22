package utils

import (
	"os"
	"runtime"
)

func IsUserRoot() bool {
	return os.Getuid() == 0
}

func IsAMD64() bool {
	return runtime.GOARCH == "amd64"
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}
