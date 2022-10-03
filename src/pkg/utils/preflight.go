package utils

import (
	"fmt"
	"os"
	"regexp"
)

func ValidHostname(hostname string) bool {
	// Explanation: https://regex101.com/r/zUGqjP/1/
	rfcDomain := regexp.MustCompile(`^[a-zA-Z0-9\-.]+$`)
	// Explanation: https://regex101.com/r/vPGnzR/1/
	localhost := regexp.MustCompile(`\.?localhost$`)
	isValid := rfcDomain.MatchString(hostname)
	if isValid {
		isValid = !localhost.MatchString(hostname)
	}
	return isValid
}

func IsValidHostName() bool {
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return ValidHostname(hostname)
}

func IsRHEL() bool {
	return !InvalidPath("/etc/redhat-release")
}

func RunPreflightChecks() error {
	if !IsValidHostName() {
		return fmt.Errorf("Please ensure this hostname is valid according to https://www.ietf.org/rfc/rfc1123.txt.")
	}

	return nil
}
