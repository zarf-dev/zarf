package gval

import (
	"io"
	"strconv"
	"text/scanner"
)

type defaultScanner struct {
	scanner.Scanner
}

// Scanner is an abstraction of the scanner used for parsing.
type Scanner interface {
	// Init initializes a Scanner with a new source.
	Init(reader io.Reader)
	// Error is called for each error encountered.
	SetError(func(Scanner, string))
	// SetFilename sets the filename in the current position.
	SetFilename(string)
	// SetWhitespace controls which characters are recognized
	// as white space. To recognize a character ch <= ' ' as white space,
	// set the ch'th bit in Whitespace (the Scanner's behavior is undefined
	// for values ch > ' ').
	SetWhitespace(uint64)
	// GetWhitespace returns the current whitespace bit mask.
	GetWhitespace() uint64
	// SetMode controls which tokens are recognized. For instance,
	// to recognize Ints, set the ScanInts bit in Mode.
	SetMode(uint)
	// GetMode returns the current bit mask of the recognized tokens.
	GetMode() uint
	// SetIsIdentRune sets a predicate controlling the characters accepted
	// as the ith rune in an identifier. The set of valid characters
	// must not intersect with the set of white space characters.
	// If no IsIdentRune function is set, regular Go identifiers are
	// accepted instead.
	SetIsIdentRune(func(rune, int) bool)
	// GetIsIdentRune returns the current function to accept runes
	// in identifiers.
	GetIsIdentRune() func(rune, int) bool
	// Scan reads the next token or Unicode character from source and returns it.
	Scan() rune
	// Peek returns the next Unicode character in the source without advancing
	// the scanner. It returns EOF if the scanner's position is at the last
	// character of the source.
	Peek() rune
	// Next reads and returns the next Unicode character.
	// It returns EOF at the end of the source.
	// It reports a read error by calling set function registed by SetError.
	// Next does not update the Scanner's Position field;
	// use Pos to get the current position.
	Next() rune
	// TokenText returns the string corresponding to the most recently
	// scanned token.
	// Valid after calling Scan and in calls of the function registered
	// to SetError.
	TokenText() string
	// Pos returns the position of the character immediately after
	// the character or token returned by the last call to Next or Scan.
	// Use GetPosition for the start position of the most recently scanned token.
	Pos() scanner.Position
	// GetPosition returns the position of most recently scanned token;
	// set by Scan.
	// Calling Init or Next invalidates the position (Line == 0).
	// The Filename field is always left untouched by the Scanner.
	// If an error is reported (via the function registered to SetError)
	// and Position is invalid, the scanner is not inside a token.
	// Call Pos to obtain an error position in that case,
	// or to obtain the position immediately after the most recently
	// scanned token.
	GetPosition() scanner.Position
	// Unquote interprets s as a single-quoted, double-quoted,
	// or backquoted Go string literal, returning the string value that s quotes.
	Unquote(string) (string, error)
}

func (s *defaultScanner) Init(reader io.Reader) {
	s.Scanner.Init(reader)
}

func (s *defaultScanner) SetError(fn func(s Scanner, msg string)) {
	if fn == nil {
		s.Scanner.Error = nil
	} else {
		s.Scanner.Error = func(_ *scanner.Scanner, msg string) {
			fn(s, msg)
		}
	}
}

func (s *defaultScanner) SetFilename(filename string) {
	s.Scanner.Filename = filename
}

func (s *defaultScanner) SetWhitespace(ws uint64) {
	s.Scanner.Whitespace = ws
}

func (s *defaultScanner) GetWhitespace() uint64 {
	return s.Scanner.Whitespace
}

func (s *defaultScanner) SetMode(m uint) {
	s.Scanner.Mode = m
}

func (s *defaultScanner) GetMode() uint {
	return s.Scanner.Mode
}

func (s *defaultScanner) SetIsIdentRune(fn func(ch rune, i int) bool) {
	s.Scanner.IsIdentRune = fn
}

func (s *defaultScanner) GetIsIdentRune() func(ch rune, i int) bool {
	return s.Scanner.IsIdentRune
}

func (s *defaultScanner) GetPosition() scanner.Position {
	return s.Scanner.Position
}

func (*defaultScanner) Unquote(s string) (string, error) {
	return strconv.Unquote(s)
}
