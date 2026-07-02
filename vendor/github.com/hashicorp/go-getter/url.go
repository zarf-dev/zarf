// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package getter

import "net/url"

// redactedParams is the list of URL query parameter names whose values are
// sensitive and must be replaced with "redacted" in error messages and logs.
var redactedParams = []string{
	"sshkey",
	"aws_access_key_id",
	"aws_access_key_secret",
	"aws_access_token",
}

// RedactURL is a port of url.Redacted from the standard library,
// which is like url.String but replaces any password with "redacted".
// Only the password in u.URL is redacted. This allows the library
// to maintain compatibility with go1.14.
// This port was also extended to redact sensitive URL query parameters
// (sshkey, aws_access_key_id, aws_access_key_secret, aws_access_token)
// and replace them with "redacted".
func RedactURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	ru := *u
	if _, has := ru.User.Password(); has {
		ru.User = url.UserPassword(ru.User.Username(), "redacted")
	}
	q := ru.Query()
	modified := false
	for _, param := range redactedParams {
		if q.Has(param) {
			values := q[param]
			for i := range values {
				values[i] = "redacted"
			}
			modified = true
		}
	}
	if modified {
		ru.RawQuery = q.Encode()
	}
	return ru.String()
}
