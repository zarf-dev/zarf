// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package helpers

import (
	"crypto/rand"
)

// Very limited special chars for git / basic auth
// https://owasp.org/www-community/password-special-characters has complete list of safe chars.
const randomStringChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!~-"

// RandomString generates a secure random string of the specified length.
func RandomString(length int) (string, error) {
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = randomStringChars[b%byte(len(randomStringChars))]
	}

	return string(bytes), nil
}
