package expr

//go:generate stringer -type=tokenkind

import (
	"fmt"
	"io"
	"strconv"
	"unicode"
	"unicode/utf8"
)

func lower(ch rune) rune {
	return ('a' - 'A') | ch
}

func isdecimal(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func isoctal(ch rune) bool {
	return '0' <= ch && ch <= '7'
}

func ishex(ch rune) bool {
	return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f'
}

func isletter(c rune) bool {
	return 'a' <= lower(c) && lower(c) <= 'z' || c == '_' || c >= utf8.RuneSelf && unicode.IsLetter(c)
}

func isdigit(c rune) bool {
	return isdecimal(c) || c >= utf8.RuneSelf && unicode.IsDigit(c)
}

func isnumber(c rune) bool {
	return isdigit(c) || ishex(c) || c == '.' || lower(c) == 'x'
}

func isident(c rune) bool {
	return isletter(c) || isdigit(c)
}

func iswhitespace(c rune) bool {
	return c == ' ' || c == '\t'
}

// tokenkind is an enumeration of different kinds of tokens.
type tokenkind int

// This is a definition of all possible token kinds.
const (
	niltoken tokenkind = iota
	errtoken
	eoftoken

	identtoken
	inttoken
	floattoken
	booltoken
	strtoken
	runetoken

	addtoken
	subtoken
	multoken
	quotoken
	remtoken

	andtoken
	nottoken
	ortoken
	xortoken
	shltoken
	shrtoken
	andnottoken

	logicalandtoken
	logicalortoken

	equaltoken
	lessertoken
	greatertoken
	notequaltoken
	lesserequaltoken
	greaterequaltoken

	leftparentoken
	leftbrackettoken
	commatoken
	periodtoken

	rightparentoken
	rightbrackettoken
	colontoken
	ternarytoken

	boolkeyword
	bytekeyword
	float32keyword
	float64keyword
	intkeyword
	int8keyword
	int16keyword
	int32keyword
	int64keyword
	uintkeyword
	uint8keyword
	uint16keyword
	uint32keyword
	uint64keyword
	uintptrkeyword
	nilkeyword
)

var keywordmap = map[string]tokenkind{
	"bool":    boolkeyword,
	"byte":    bytekeyword,
	"float32": float32keyword,
	"float64": float64keyword,
	"int":     intkeyword,
	"int8":    int8keyword,
	"int16":   int16keyword,
	"int32":   int32keyword,
	"int64":   int64keyword,
	"uint":    uintkeyword,
	"uint8":   uint8keyword,
	"uint16":  uint16keyword,
	"uint32":  uint32keyword,
	"uint64":  uint64keyword,
	"uintptr": uintptrkeyword,
	"nil":     nilkeyword,
}

const eof = utf8.MaxRune + 0x0001

// token contains information for a single lexical token.
type token struct {
	kind tokenkind
	pos  int

	sval string
	ival int64
	uval uint64
	fval float64
	bval bool
	sign bool
	eval error
}

// scanner scans lexical tokens from the expression.
type scanner struct {
	r   io.RuneScanner
	p   int
	eof bool
}

func newscanner(r io.RuneScanner) *scanner {
	return &scanner{r: r}
}

func (s *scanner) readrune() rune {
	if s.eof {
		return eof
	}
	c, _, err := s.r.ReadRune()
	if err == io.EOF {
		s.eof = true
		return eof
	} else if err != nil {
		panic(err)
	}
	s.p++
	return c
}

func (s *scanner) unreadrune() {
	if s.eof {
		return
	}
	if err := s.r.UnreadRune(); err != nil {
		panic(err)
	}
	s.p--
}

func (s *scanner) skipws() {
	for {
		c := s.readrune()
		if !iswhitespace(c) {
			s.unreadrune()
			return
		}
	}
}

func (s *scanner) accept(c rune) bool {
	if s.readrune() == c {
		return true
	}
	s.unreadrune()
	return false
}

func (s *scanner) expect(c rune) {
	r := s.readrune()
	if r != c {
		panic(fmt.Errorf("expected %c", r))
	}
}

func (s *scanner) peekmatch(f func(rune) bool) bool {
	c := s.readrune()
	s.unreadrune()
	return f(c)
}

func (s *scanner) acceptfn(f func(rune) bool) (rune, bool) {
	r := s.readrune()
	if f(r) {
		return r, true
	}
	s.unreadrune()
	return r, false
}

func (s *scanner) expectfn(f func(rune) bool) rune {
	r, ok := s.acceptfn(f)
	if !ok {
		panic(fmt.Errorf("unexpected %c", r))
	}
	return r
}

func (s *scanner) tokensym(k tokenkind, src string) token {
	return token{kind: k, sval: src}
}

func (s *scanner) errsymf(format string, a ...interface{}) token {
	return token{kind: errtoken, eval: fmt.Errorf(format, a...)}
}

func (s *scanner) scanident() token {
	t := token{kind: identtoken}
	if r, ok := s.acceptfn(isletter); ok {
		t.sval = string(r)
	} else {
		return s.errsymf("unexpected ident start token: %c", r)
	}
	for {
		if r, ok := s.acceptfn(isident); ok {
			t.sval += string(r)
			continue
		}
		break
	}
	// Handle boolean constant.
	if t.sval == "true" {
		t.kind = booltoken
		t.bval = true
	}
	if t.sval == "false" {
		t.kind = booltoken
		t.bval = false
	}
	// Handle keywords.
	if k, ok := keywordmap[t.sval]; ok {
		t.kind = k
	}
	return t
}

func (s *scanner) scannumber(t token) token {
	if r, ok := s.acceptfn(isnumber); ok {
		t.sval += string(r)
	} else {
		return s.errsymf("unexpected int start token: %c", r)
	}
	for {
		if r, ok := s.acceptfn(isnumber); ok {
			t.sval += string(r)
			continue
		}
		break
	}
	var err error
	if t.uval, err = strconv.ParseUint(t.sval, 0, 64); err == nil {
		t.ival = int64(t.uval)
		t.fval = float64(t.ival)
		t.kind = inttoken
		t.sign = false
	} else if t.ival, err = strconv.ParseInt(t.sval, 0, 64); err == nil {
		t.uval = uint64(t.ival)
		t.fval = float64(t.ival)
		t.kind = inttoken
		t.sign = true
	} else if t.fval, err = strconv.ParseFloat(t.sval, 64); err == nil {
		t.ival = int64(t.fval)
		t.uval = uint64(t.fval)
		t.kind = floattoken
	} else {
		return token{kind: errtoken, eval: err}
	}
	return t
}

func runebytes(r rune) []byte {
	runebuf := [4]byte{}
	l := utf8.EncodeRune(runebuf[:], rune(r))
	return runebuf[:l]
}

func (s *scanner) scanescape(quote byte) []byte {
	switch {
	case s.accept('a'):
		return []byte{'\a'}
	case s.accept('b'):
		return []byte{'\b'}
	case s.accept('f'):
		return []byte{'\f'}
	case s.accept('n'):
		return []byte{'\n'}
	case s.accept('r'):
		return []byte{'\r'}
	case s.accept('t'):
		return []byte{'\t'}
	case s.accept('v'):
		return []byte{'\v'}
	case s.accept('\\'):
		return []byte{'\\'}
	case s.accept(rune(quote)):
		return []byte{quote}
	case s.peekmatch(isoctal):
		octal := string([]rune{s.expectfn(isoctal), s.expectfn(isoctal), s.expectfn(isoctal)})
		code, err := strconv.ParseUint(octal, 8, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case s.accept('x'):
		hex := string([]rune{s.expectfn(ishex), s.expectfn(ishex)})
		code, err := strconv.ParseUint(hex, 16, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case s.accept('u'):
		hex := string([]rune{
			s.expectfn(ishex), s.expectfn(ishex), s.expectfn(ishex), s.expectfn(ishex),
		})
		code, err := strconv.ParseUint(hex, 16, 16)
		if err != nil {
			panic(err)
		}
		return runebytes(rune(code))
	case s.accept('U'):
		hex := string([]rune{
			s.expectfn(ishex), s.expectfn(ishex), s.expectfn(ishex), s.expectfn(ishex),
			s.expectfn(ishex), s.expectfn(ishex), s.expectfn(ishex), s.expectfn(ishex),
		})
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			panic(err)
		}
		return runebytes(rune(code))
	default:
		panic(fmt.Errorf("unexpected escape code %c", s.readrune()))
	}
}

func (s *scanner) scanstring(quote byte) token {
	str := []byte{}
	for {
		switch {
		case s.accept(rune(quote)):
			return token{kind: strtoken, sval: string(str)}
		case s.accept('\\'):
			str = append(str, s.scanescape(quote)...)
		default:
			str = append(str, runebytes(s.readrune())...)
		}
	}
}

func (s *scanner) scanrune() token {
	defer s.expect('\'')
	switch {
	case s.accept('\\'):
		r := []rune(string(s.scanescape('\'')))
		return token{kind: runetoken, ival: int64(r[0])}
	default:
		return token{kind: runetoken, ival: int64(s.readrune())}
	}
}

func (s *scanner) scan() (t token) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				t = token{kind: errtoken, pos: s.p, eval: err}
			} else {
				panic(r)
			}
		}
	}()

	s.skipws()

	p := s.p

	defer func() {
		t.pos = p
	}()

	switch {
	case s.accept(eof):
		p = s.p
		return token{kind: eoftoken}
	case s.peekmatch(isletter):
		return s.scanident()
	case s.peekmatch(isdigit):
		return s.scannumber(token{})
	case s.accept('"'):
		return s.scanstring('"')
	case s.accept('\''):
		return s.scanrune()
	case s.accept('$'):
		s.expect('\'')
		return s.scanstring('\'')
	case s.accept('+'):
		return s.tokensym(addtoken, "+")
	case s.accept('-'):
		switch {
		default:
			return s.tokensym(subtoken, "-")
		case s.peekmatch(isdigit):
			return s.scannumber(token{sval: "-"})
		}
	case s.accept('*'):
		return s.tokensym(multoken, "*")
	case s.accept('/'):
		return s.tokensym(quotoken, "/")
	case s.accept('%'):
		return s.tokensym(remtoken, "%")
	case s.accept('&'):
		switch {
		default:
			return s.tokensym(andtoken, "&")
		case s.accept('&'):
			return s.tokensym(logicalandtoken, "&&")
		case s.accept('^'):
			return s.tokensym(andnottoken, "&^")
		}
	case s.accept('|'):
		switch {
		default:
			return s.tokensym(ortoken, "|")
		case s.accept('|'):
			return s.tokensym(logicalortoken, "||")
		}
	case s.accept('^'):
		switch {
		default:
			return s.tokensym(xortoken, "^")
		}
	case s.accept('<'):
		switch {
		default:
			return s.tokensym(lessertoken, "<")
		case s.accept('='):
			return s.tokensym(lesserequaltoken, "<=")
		case s.accept('<'):
			return s.tokensym(shltoken, "<<")
		}
	case s.accept('>'):
		switch {
		default:
			return s.tokensym(greatertoken, ">")
		case s.accept('='):
			return s.tokensym(greaterequaltoken, ">=")
		case s.accept('>'):
			return s.tokensym(shrtoken, ">>")
		}
	case s.accept('='):
		switch {
		case s.accept('='):
			return s.tokensym(equaltoken, "==")
		default:
			return s.errsymf("unexpected rune %c", s.readrune())
		}
	case s.accept('!'):
		switch {
		case s.accept('='):
			return s.tokensym(notequaltoken, "!=")
		default:
			return s.tokensym(nottoken, "!")
		}
	case s.accept('('):
		return s.tokensym(leftparentoken, "(")
	case s.accept('['):
		return s.tokensym(leftbrackettoken, "[")
	case s.accept(','):
		return s.tokensym(commatoken, ",")
	case s.accept('.'):
		switch {
		default:
			return s.tokensym(periodtoken, ".")
		case s.peekmatch(isdigit):
			return s.scannumber(token{sval: "."})
		}
	case s.accept(')'):
		return s.tokensym(rightparentoken, ")")
	case s.accept(']'):
		return s.tokensym(rightbrackettoken, "]")
	case s.accept(':'):
		return s.tokensym(colontoken, ":")
	case s.accept('?'):
		return s.tokensym(ternarytoken, "?")
	default:
		return s.errsymf("unexpected rune %c", s.readrune())
	}
}
