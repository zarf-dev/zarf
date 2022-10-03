package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func ValidateSha256Sum(expectedChecksum string, path string) bool {
	actualChecksum, _ := GetSha256Sum(path)
	return expectedChecksum == actualChecksum
}

// GetSha256Sum returns the computed SHA256 Sum of a given file
func GetSha256Sum(path string) (string, error) {
	var data io.ReadCloser
	var err error

	if IsUrl(path) {
		// Handle download from URL
		data, err = Fetch(path)
		if err != nil {
			return "", err
		}
	} else {
		// Handle local file
		data, err = os.Open(path)
		if err != nil {
			return "", err
		}
	}

	defer data.Close()

	hash := sha256.New()
	_, err = io.Copy(hash, data)

	if err != nil {
		return "", err
	} else {
		computedHash := hex.EncodeToString(hash.Sum(nil))
		return computedHash, nil
	}
}
