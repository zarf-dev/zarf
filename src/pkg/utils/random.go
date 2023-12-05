// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"crypto/rand"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/fatih/color"
)

// Very limited special chars for git / basic auth
// https://owasp.org/www-community/password-special-characters has complete list of safe chars.
const randomStringChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!~-"

// RandomString generates a secure random string of the specified length.
func RandomString(length int) string {
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		message.Fatal(err, "unable to generate a random secret")
	}

	for i, b := range bytes {
		bytes[i] = randomStringChars[b%byte(len(randomStringChars))]
	}

	return string(bytes)
}

// First30last30 returns the source string that has been trimmed to 30 characters at the beginning and end.
func First30last30(s string) string {
	if len(s) > 60 {
		return s[0:27] + "..." + s[len(s)-26:]
	}

	return s
}

// ColorWrap changes a string to an ansi color code and appends the default color to the end
// preventing future characters from taking on the given color
// returns string as normal if color is disabled
func ColorWrap(str string, attr color.Attribute) string {
	if config.NoColor {
		return str
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", attr, str)
}
