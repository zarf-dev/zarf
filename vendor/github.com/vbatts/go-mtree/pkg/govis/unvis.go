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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var (
	errEndOfString       = errors.New("unexpectedly reached end of string")
	errUnknownEscapeChar = errors.New("unknown escape character")
	errOutsideLatin1     = errors.New("outside latin-1 encoding")
	errParseDigit        = errors.New("could not parse digit")
)

// unvisParser stores the current state of the token parser.
type unvisParser struct {
	output *strings.Builder
	tokens []rune
	idx    int
	flags  VisFlag
}

// Input resets the parser with a new input string.
func (p *unvisParser) Input(input string) {
	p.output = new(strings.Builder)
	p.output.Grow(len(input)) // the output will be at most input-sized

	p.tokens = []rune(input)
	p.idx = 0
}

// Output returns the internal [strings.Builder].
func (p *unvisParser) Output() *strings.Builder {
	return p.output
}

// Step moves the index to the next character.
func (p *unvisParser) Step() {
	p.idx++
}

// Peek gets the current token.
func (p *unvisParser) Peek() (rune, error) {
	if p.idx >= len(p.tokens) {
		return unicode.ReplacementChar, errEndOfString
	}
	return p.tokens[p.idx], nil
}

// Next moves the index to the next character and returns said character.
func (p *unvisParser) Next() (rune, error) {
	ch, err := p.Peek()
	if err == nil {
		p.Step()
	}
	return ch, err
}

// End returns whether all of the tokens have been consumed.
func (p *unvisParser) End() bool {
	return p.idx >= len(p.tokens)
}

func newParser(flags VisFlag) *unvisParser {
	return &unvisParser{
		output: nil,
		tokens: nil,
		idx:    0,
		flags:  flags,
	}
}

// While a recursive descent parser is overkill for parsing simple escape
// codes, this is IMO much easier to read than the ugly 80s coroutine code used
// by the original unvis(3) parser. Here's the EBNF for an unvis sequence:
//
// <input>           ::= (<element>)*
// <element>         ::= ("\" <escape-sequence>) | ("%" <escape-hex>) | <plain-rune>
// <plain-rune>      ::= any rune
// <escape-sequence> ::= ("x" <escape-hex>) | ("M" <escape-meta>) | ("^" <escape-ctrl) | <escape-cstyle> | <escape-octal>
// <escape-meta>     ::= ("-" <escape-meta1>) | ("^" <escape-ctrl>)
// <escape-meta1>    ::= any rune
// <escape-ctrl>     ::= "?" | any rune
// <escape-cstyle>   ::= "\" | "n" | "r" | "b" | "a" | "v" | "t" | "f"
// <escape-hex>      ::= [0-9a-f] [0-9a-f]
// <escape-octal>    ::= [0-7] ([0-7] ([0-7])?)?

func (p *unvisParser) plainRune() error {
	ch, err := p.Next()
	if err != nil {
		return fmt.Errorf("plain rune: %w", err)
	}
	_, err = p.output.WriteRune(ch)
	return err
}

func (p *unvisParser) escapeCStyle() error {
	ch, err := p.Next()
	if err != nil {
		return fmt.Errorf("escape cstyle: %w", err)
	}

	switch ch {
	case 'n':
		return p.output.WriteByte('\n')
	case 'r':
		return p.output.WriteByte('\r')
	case 'b':
		return p.output.WriteByte('\b')
	case 'a':
		return p.output.WriteByte('\x07')
	case 'v':
		return p.output.WriteByte('\v')
	case 't':
		return p.output.WriteByte('\t')
	case 'f':
		return p.output.WriteByte('\f')
	case 's':
		return p.output.WriteByte(' ')
	case 'E':
		return p.output.WriteByte('\x1b')
	case '\n', '$':
		// Hidden newline or marker.
		return nil
	}
	// XXX: We should probably allow falling through and return "\" here...
	return fmt.Errorf("escape cstyle: %w %q", errUnknownEscapeChar, ch)
}

func (p *unvisParser) escapeDigits(base int, force bool) error {
	var code int
	for i := int(0xFF); i > 0; i /= base {
		ch, err := p.Peek()
		if err != nil {
			if !force && i != 0xFF {
				break
			}
			return fmt.Errorf("escape base %d: %w", base, err)
		}

		digit, err := strconv.ParseInt(string(ch), base, 8)
		if err != nil {
			if !force && i != 0xFF {
				break
			}
			return fmt.Errorf("escape base %d: %w %q: %w", base, errParseDigit, ch, err)
		}

		code = (code * base) + int(digit)
		p.Step() // only consume token if we use it (length is variable)
	}
	if code > unicode.MaxLatin1 {
		return fmt.Errorf("escape base %d: code %+.2x %w", base, code, errOutsideLatin1)
	}
	return p.output.WriteByte(byte(code))
}

func (p *unvisParser) escapeCtrl(mask byte) error {
	ch, err := p.Next()
	if err != nil {
		return fmt.Errorf("escape ctrl: %w", err)
	}
	if ch > unicode.MaxLatin1 {
		return fmt.Errorf("escape ctrl: code %q %w", ch, errOutsideLatin1)
	}
	char := byte(ch) & 0x1f
	if ch == '?' {
		char = 0x7f
	}
	return p.output.WriteByte(mask | char)
}

func (p *unvisParser) escapeMeta() error {
	ch, err := p.Next()
	if err != nil {
		return fmt.Errorf("escape meta: %w", err)
	}

	mask := byte(0x80)
	switch ch {
	case '^':
		// The same as "\^..." except we apply a mask.
		return p.escapeCtrl(mask)

	case '-':
		ch, err := p.Next()
		if err != nil {
			return fmt.Errorf("escape meta1: %w", err)
		}
		if ch > unicode.MaxLatin1 {
			return fmt.Errorf("escape meta1: code %q %w", ch, errOutsideLatin1)
		}
		// Add mask to character.
		return p.output.WriteByte(mask | byte(ch))
	}

	return fmt.Errorf("escape meta: %w %q", errUnknownEscapeChar, ch)
}

func (p *unvisParser) escapeSequence() error {
	ch, err := p.Peek()
	if err != nil {
		return fmt.Errorf("escape sequence: %w", err)
	}

	switch ch {
	case '\\', '"':
		p.Step()
		return p.output.WriteByte(byte(ch))

	case '0', '1', '2', '3', '4', '5', '6', '7':
		return p.escapeDigits(8, false)

	case 'x':
		p.Step()
		return p.escapeDigits(16, true)

	case '^':
		p.Step()
		return p.escapeCtrl(0x00)

	case 'M':
		p.Step()
		return p.escapeMeta()

	default:
		return p.escapeCStyle()
	}
}

func (p *unvisParser) element() error {
	ch, err := p.Peek()
	if err != nil {
		return err
	}

	switch ch {
	case '\\':
		p.Step()
		return p.escapeSequence()

	case '%':
		// % HEX HEX only applies to HTTPStyle encodings.
		if p.flags&VisHTTPStyle == VisHTTPStyle {
			p.Step()
			return p.escapeDigits(16, true)
		}
	}
	return p.plainRune()
}

func (p *unvisParser) unvis(input string) (string, error) {
	p.Input(input)
	for !p.End() {
		if err := p.element(); err != nil {
			return "", err
		}
	}
	return p.Output().String(), nil
}

// Unvis takes a string formatted with the given Vis flags (though only the
// VisHTTPStyle flag is checked) and output the un-encoded version of the
// encoded string. An error is returned if any escape sequences in the input
// string were invalid.
func Unvis(input string, flags VisFlag) (string, error) {
	if unknown := flags &^ visMask; unknown != 0 {
		return "", unknownVisFlagsError{flags: flags}
	}
	p := newParser(flags)
	output, err := p.unvis(input)
	if err != nil {
		return "", fmt.Errorf("unvis '%s': %w", input, err)
	}
	return output, nil
}
