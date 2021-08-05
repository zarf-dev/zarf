package utils

import (
	"os"
	"regexp"
	"runtime"

	"github.com/sirupsen/logrus"
)

func CheckHostName(hostname string) bool {
	expression := regexp.MustCompile(`^[a-zA-Z0-9\-.]+$`)
	return expression.MatchString(hostname)
}

func IsValidHostName() bool {
	logrus.Info("Preflight check: validating hostname")
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return CheckHostName(hostname)
}

func IsUserRoot() bool {
	logrus.Info("Preflight check: validating user is root")
	return os.Getuid() == 0
}

func IsAMD64() bool {
	logrus.Info("Preflight check: validating AMD64 arch")
	return runtime.GOARCH == "amd64"
}

func IsLinux() bool {
	logrus.Info("Preflight check: validating os type")
	return runtime.GOOS == "linux"
}

func IsRHEL() bool {
	return !InvalidPath("/etc/redhat-release")
}

func RunPreflightChecks() {
	if !IsLinux() {
		logrus.Fatal("This program requires a Linux OS")
	}

	if !IsAMD64() {
		logrus.Fatal("This program currently only runs on AMD64 architectures")
	}

	if !IsUserRoot() {
		logrus.Fatal("You must run this program as root.")
	}

	if !IsValidHostName() {
		logrus.Fatal("Please ensure this hostname is valid according to https://www.ietf.org/rfc/rfc1123.txt.")
	}
}
