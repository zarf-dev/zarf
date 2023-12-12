// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
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
		//message.Fatal(err, "unable to generate a random secret")
		return "", err
	}

	for i, b := range bytes {
		bytes[i] = randomStringChars[b%byte(len(randomStringChars))]
	}

	return string(bytes), nil
}
