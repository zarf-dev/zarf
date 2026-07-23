// SPDX-License-Identifier: Apache-2.0
/*
 * govis: unicode aware vis(3) encoding implementation
 * Copyright (C) 2017-2025 SUSE LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package govis

import (
	"fmt"
	"strings"
	"unicode"
)

var maxAscii byte = unicode.MaxASCII // 0x7f

func isunsafe(ch byte) bool {
	return ch == '\b' || ch == '\007' || ch == '\r'
}

func isglob(ch byte) bool {
	return ch == '*' || ch == '?' || ch == '[' || ch == '#'
}

// ishttp is defined by RFC 1808.
func ishttp(ch byte) bool {
	// RFC1808 does not really consider characters outside of ASCII, so just to
	// be safe always treat characters outside the ASCII character set as "not
	// HTTP".
	if ch > maxAscii {
		return false
	}
	return unicode.IsDigit(rune(ch)) || unicode.IsLetter(rune(ch)) ||
		// Safe characters.
		ch == '$' || ch == '-' || ch == '_' || ch == '.' || ch == '+' ||
		// Extra characters.
		ch == '!' || ch == '*' || ch == '\'' || ch == '(' ||
		ch == ')' || ch == ','
}

func isgraph(ch byte) bool {
	return ch <= maxAscii &&
		unicode.IsGraphic(rune(ch)) && !unicode.IsSpace(rune(ch))
}

func isctrl(ch byte) bool {
	return unicode.IsControl(rune(ch))
}

// vis converts a single *byte* into its encoding. While Go supports the
// concept of runes (and thus native utf-8 parsing), in order to make sure that
// the bit-stream will be completely maintained through an Unvis(Vis(...))
// round-trip. The downside is that Vis() will never output unicode -- but on
// the plus side this is actually a benefit on the encoding side (it will
// always work with the simple unvis(3) implementation). It also means that we
// don't have to worry about different multi-byte encodings.
func vis(output *strings.Builder, ch byte, flag VisFlag) {
	// XXX: This is quite a horrible thing to support.
	if flag&VisHTTPStyle == VisHTTPStyle && !ishttp(ch) {
		_, _ = fmt.Fprintf(output, "%%%.2X", ch)
		return
	}

	// Figure out if the character doesn't need to be encoded. Effectively, we
	// encode most "normal" (graphical) characters as themselves unless we have
	// been specifically asked not to.
	switch {
	case ch > maxAscii:
		// We must *always* encode stuff characters not in ASCII.
	case flag&VisGlob == VisGlob && isglob(ch):
		// Glob characters are graphical but can be forced to be encoded.
	case flag&VisNoSlash == 0 && ch == '\\',
		flag&VisDoubleQuote == VisDoubleQuote && ch == '"':
		// Prefix \ if applicable.
		_ = output.WriteByte('\\')
		fallthrough
	case isgraph(ch),
		flag&VisSpace != VisSpace && ch == ' ',
		flag&VisTab != VisTab && ch == '\t',
		flag&VisNewline != VisNewline && ch == '\n',
		flag&VisSafe != 0 && isunsafe(ch):
		_ = output.WriteByte(ch)
		return
	}

	// Try to use C-style escapes first.
	if flag&VisCStyle == VisCStyle {
		switch ch {
		case ' ':
			_, _ = output.WriteString("\\s")
			return
		case '\n':
			_, _ = output.WriteString("\\n")
			return
		case '\r':
			_, _ = output.WriteString("\\r")
			return
		case '\b':
			_, _ = output.WriteString("\\b")
			return
		case '\a':
			_, _ = output.WriteString("\\a")
			return
		case '\v':
			_, _ = output.WriteString("\\v")
			return
		case '\t':
			_, _ = output.WriteString("\\t")
			return
		case '\f':
			_, _ = output.WriteString("\\f")
			return
		case '\x00':
			// Output octal just to be safe.
			_, _ = output.WriteString("\\000")
			return
		}
	}

	// For graphical characters we generate octal output (and also if it's
	// being forced by the caller's flags). Also spaces should always be
	// encoded as octal (note that ' '|0x80 == '\xa0' is a non-breaking space).
	if flag&VisOctal == VisOctal || isgraph(ch) || ch&0x7f == ' ' {
		// Always output three-character octal just to be safe.
		_, _ = fmt.Fprintf(output, "\\%.3o", ch)
		return
	}

	// Now we have to output meta or ctrl escapes. As far as I can tell, this
	// is not actually defined by any standard -- so this logic is basically
	// copied from the original vis(3) implementation. Hopefully nobody
	// actually relies on this (octal and hex are better).

	if flag&VisNoSlash == 0 {
		_ = output.WriteByte('\\')
	}

	// Meta characters have 0x80 set, but are otherwise identical to control
	// characters.
	if ch&0x80 != 0 {
		ch &= 0x7f
		_ = output.WriteByte('M')
	}
	if isctrl(ch) {
		_ = output.WriteByte('^')
		if ch == 0x7f {
			_ = output.WriteByte('?')
		} else {
			_ = output.WriteByte(ch + '@')
		}
	} else {
		_ = output.WriteByte('-')
		_ = output.WriteByte(ch)
	}
}

// Vis encodes the provided string to a BSD-compatible encoding using BSD's
// vis() flags. However, it will correctly handle multi-byte encoding (which is
// not done properly by BSD's vis implementation).
func Vis(src string, flags VisFlag) (string, error) {
	if unknown := flags &^ visMask; unknown != 0 {
		return "", unknownVisFlagsError{flags: flags}
	}
	var output strings.Builder
	output.Grow(len(src)) // vis() will always take up at least len(src) bytes
	for _, ch := range []byte(src) {
		vis(&output, ch, flags)
	}
	return output.String(), nil
}
