package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func ValidateSha256Sum(expectedChecksum string, path string) {
	actualChecksum, _ := GetSha256Sum(path)
	if expectedChecksum != actualChecksum {
		logrus.WithFields(logrus.Fields{
			"Source":   path,
			"Expected": expectedChecksum,
			"Actual":   actualChecksum,
		}).Fatal("Invalid or mismatched file checksum")
	}
}

// GetSha256Sum returns the computed SHA256 Sum of a given file
func GetSha256Sum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, file)

	if err != nil {
		return "", err
	} else {
		computedHash := hex.EncodeToString(hash.Sum(nil))
		return computedHash, nil
	}
}
