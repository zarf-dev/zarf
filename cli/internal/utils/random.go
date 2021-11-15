package utils

import (
	"crypto/rand"

	"github.com/sirupsen/logrus"
)

// Very limited special chars for git / basic auth
// https://owasp.org/www-community/password-special-characters has complete list of safe chars
const randomStringChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!~-"

func RandomString(length int) string {
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		logrus.Debug(err)
		logrus.Fatal("unable to generate a random secret")
	}

	for i, b := range bytes {
		bytes[i] = randomStringChars[b%byte(len(randomStringChars))]
	}

	return string(bytes)
}
