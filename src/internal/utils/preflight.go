package utils

import (
	"os"
	"runtime"
	"regexp"

	log "github.com/sirupsen/logrus"
)

// injected checksum for the tarball bundled with this binary
var packageChecksum string

func IsValidHostName() bool {
	log.Info("Preflight check: validating hostname")
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	expression := regexp.MustCompile(`^[a-zA-Z0-9\-\.]`)
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return expression.MatchString(hostname)
}

func IsUserRoot() bool {
	log.Info("Preflight check: validating user is root")
	return os.Getuid() == 0
}

func IsAMD64() bool {
	log.Info("Preflight check: validating AMD64 arch")
	return runtime.GOARCH == "amd64"
}

func IsLinux() bool {
	log.Info("Preflight check: validating os type")
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

	if !IsValidHostName() {
		log.Fatal("Please ensure this hostname is valid according to https://www.ietf.org/rfc/rfc1123.txt.")
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
