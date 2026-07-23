package jsonpath

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// This is mainly taken from
//
// https://cs.opensource.google/go/go/+/refs/tags/go1.21.0:src/strconv/quote.go
//
// and adjusted to meet the needs of unquoting JSONPath strings.
// Mainly handling single quoted strings right and removed support for
// Go raw strings.

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// contains reports whether the string contains the byte c.
func contains(s string, c byte) bool {
	return strings.IndexByte(s, c) != -1
}

// unquote interprets s as a single-quoted, double-quoted,
// or backquoted Go string literal, returning the string value
// that s quotes.  (If s is single-quoted, it would be a Go
// character literal; Unquote returns the corresponding
// one-character string.)
func unquote(s string) (string, error) {
	out, rem, err := unquoteInternal(s)
	if len(rem) > 0 {
		return "", strconv.ErrSyntax
	}
	return out, err
}

// unquote parses a quoted string at the start of the input,
// returning the parsed prefix, the remaining suffix, and any parse errors.
// If unescape is true, the parsed prefix is unescaped,
// otherwise the input prefix is provided verbatim.
func unquoteInternal(in string) (out, rem string, err error) {
	// In our use case it's a constant.
	const unescape = true
	// Determine the quote form and optimistically find the terminating quote.
	if len(in) < 2 {
		return "", in, strconv.ErrSyntax
	}
	quote := in[0]
	end := strings.IndexByte(in[1:], quote)
	if end < 0 {
		return "", in, strconv.ErrSyntax
	}
	end += 2 // position after terminating quote; may be wrong if escape sequences are present

	switch quote {
	case '"', '\'':
		// Handle quoted strings without any escape sequences.
		if !contains(in[:end], '\\') && !contains(in[:end], '\n') {
			var ofs int
			if quote == '\'' {
				ofs = len(`"`)
			} else {
				ofs = len(`'`)
			}
			valid := utf8.ValidString(in[ofs : end-ofs])
			if valid {
				out = in[:end]
				if unescape {
					out = out[1 : end-1] // exclude quotes
				}
				return out, in[end:], nil
			}
		}

		// Handle quoted strings with escape sequences.
		var buf []byte
		in0 := in
		in = in[1:] // skip starting quote
		if unescape {
			buf = make([]byte, 0, 3*end/2) // try to avoid more allocations
		}
		for len(in) > 0 && in[0] != quote {
			// Process the next character,
			// rejecting any unescaped newline characters which are invalid.
			r, multibyte, rem, err := strconv.UnquoteChar(in, quote)
			if in[0] == '\n' || err != nil {
				return "", in0, strconv.ErrSyntax
			}
			in = rem

			// Append the character if unescaping the input.
			if unescape {
				if r < utf8.RuneSelf || !multibyte {
					buf = append(buf, byte(r))
				} else {
					var arr [utf8.UTFMax]byte
					n := utf8.EncodeRune(arr[:], r)
					buf = append(buf, arr[:n]...)
				}
			}
		}

		// Verify that the string ends with a terminating quote.
		if !(len(in) > 0 && in[0] == quote) {
			return "", in0, strconv.ErrSyntax
		}
		in = in[1:] // skip terminating quote

		if unescape {
			return string(buf), in, nil
		}
		return in0[:len(in0)-len(in)], in, nil
	default:
		return "", in, strconv.ErrSyntax
	}
}
