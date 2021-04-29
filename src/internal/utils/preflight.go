package utils

import (
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
)

// injected checksum for the tarball bundled with this binary
var packageChecksum string

func IsUserRoot() bool {
	return os.Getuid() == 0
}

func IsAMD64() bool {
	return runtime.GOARCH == "amd64"
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}

func IsRHEL() bool {
	return !InvalidPath("/etc/redhat-release")
}

func RunPreflightChecks() {
	if !IsLinux() {
		log.Fatal("This program requires a Linux OS")
	}

	if !IsAMD64() {
		log.Fatal("This program currently only runs on AMD64 architectures")
	}

	if !IsUserRoot() {
		log.Fatal("You must run this program as root.")
	}
}

func RunTarballChecksumValidate() {
	log.Info("Validating tarball checksum")

	tarballChecksumComputed := GetSha256("shift-pack.tar.zst")

	if tarballChecksumComputed != packageChecksum {
		log.WithFields(log.Fields{
			"Computed": tarballChecksumComputed,
			"Expected": packageChecksum,
		}).Fatal("Invalid or mismatched tarball checksum")
	}

	log.Info("Tarball checksum validated")
}
