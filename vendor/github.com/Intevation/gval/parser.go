package gval

import (
	"bytes"
	"fmt"
	"strings"
	"text/scanner"
	"unicode"
)

// Parser parses expressions in a Language into an Evaluable
type Parser struct {
	scanner Scanner
	Language
	lastScan   rune
	camouflage error
}

func newParser(expression string, l Language) *Parser {

	var sc Scanner

	if l.createScanner != nil {
		sc = l.createScanner()
	} else {
		sc = &defaultScanner{}
	}
	sc.Init(strings.NewReader(expression))
	sc.SetError(func(Scanner, string) {})
	sc.SetFilename(expression + "\t")
	p := &Parser{scanner: sc, Language: l}
	p.resetScannerProperties()
	return p
}

func (p *Parser) resetScannerProperties() {
	p.scanner.SetWhitespace(scanner.GoWhitespace)
	p.scanner.SetMode(scanner.GoTokens)
	p.scanner.SetIsIdentRune(func(r rune, pos int) bool {
		return unicode.IsLetter(r) || r == '_' || (pos > 0 && unicode.IsDigit(r))
	})
}

// SetWhitespace sets the behavior of the whitespace matcher. The given
// characters must be less than or equal to 0x20 (' ').
func (p *Parser) SetWhitespace(chars ...rune) {
	var mask uint64
	for _, char := range chars {
		mask |= 1 << char
	}

	p.scanner.SetWhitespace(mask)
}

// SetMode sets the tokens that the underlying scanner will match.
func (p *Parser) SetMode(mode uint) {
	p.scanner.SetMode(mode)
}

// SetIsIdentRuneFunc sets the function that matches ident characters in the
// underlying scanner.
func (p *Parser) SetIsIdentRuneFunc(fn func(ch rune, i int) bool) {
	p.scanner.SetIsIdentRune(fn)
}

// Scan reads the next token or Unicode character from source and returns it.
// It only recognizes tokens t for which the respective Mode bit (1<<-t) is set.
// It returns scanner.EOF at the end of the source.
func (p *Parser) Scan() rune {
	if p.isCamouflaged() {
		p.camouflage = nil
		return p.lastScan
	}
	p.camouflage = nil
	p.lastScan = p.scanner.Scan()
	return p.lastScan
}

func (p *Parser) isCamouflaged() bool {
	return p.camouflage != nil && p.camouflage != errCamouflageAfterNext
}

// Camouflage rewind the last Scan(). The Parser holds the camouflage error until
// the next Scan()
// Do not call Rewind() on a camouflaged Parser
func (p *Parser) Camouflage(unit string, expected ...rune) {
	if p.isCamouflaged() {
		panic(fmt.Errorf("can only Camouflage() after Scan(): %w", p.camouflage))
	}
	p.camouflage = p.Expected(unit, expected...)
}

// Peek returns the next Unicode character in the source without advancing
// the scanner. It returns EOF if the scanner's position is at the last
// character of the source.
// Do not call Peek() on a camouflaged Parser
func (p *Parser) Peek() rune {
	if p.isCamouflaged() {
		panic("can not Peek() on camouflaged Parser")
	}
	return p.scanner.Peek()
}

var errCamouflageAfterNext = fmt.Errorf("Camouflage() after Next()")

// Next reads and returns the next Unicode character.
// It returns EOF at the end of the source.
// Do not call Next() on a camouflaged Parser
func (p *Parser) Next() rune {
	if p.isCamouflaged() {
		panic("can not Next() on camouflaged Parser")
	}
	p.camouflage = errCamouflageAfterNext
	return p.scanner.Next()
}

// TokenText returns the string corresponding to the most recently scanned token.
// Valid after calling Scan().
func (p *Parser) TokenText() string {
	return p.scanner.TokenText()
}

// Expected returns an error signaling an unexpected Scan() result
func (p *Parser) Expected(unit string, expected ...rune) error {
	return unexpectedRune{unit, expected, p.lastScan}
}

type unexpectedRune struct {
	unit     string
	expected []rune
	got      rune
}

func (err unexpectedRune) Error() string {
	exp := bytes.Buffer{}
	runes := err.expected
	switch len(runes) {
	default:
		for _, r := range runes[:len(runes)-2] {
			exp.WriteString(scanner.TokenString(r))
			exp.WriteString(", ")
		}
		fallthrough
	case 2:
		exp.WriteString(scanner.TokenString(runes[len(runes)-2]))
		exp.WriteString(" or ")
		fallthrough
	case 1:
		exp.WriteString(scanner.TokenString(runes[len(runes)-1]))
	case 0:
		return fmt.Sprintf("unexpected %s while scanning %s", scanner.TokenString(err.got), err.unit)
	}
	return fmt.Sprintf("unexpected %s while scanning %s expected %s", scanner.TokenString(err.got), err.unit, exp.String())
}
