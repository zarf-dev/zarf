package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
)

func GetSha256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}
