package restruct

import (
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func lower(ch rune) rune {
	return ('a' - 'A') | ch
}

func isdecimal(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func ishex(ch rune) bool {
	return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f'
}

func isletter(c rune) bool {
	return 'a' <= lower(c) && lower(c) <= 'z' || c == '_' || c >= utf8.RuneSelf && unicode.IsLetter(c)
}

func isdigit(c rune) bool {
	return isdecimal(c)
}

func isident(c rune) bool {
	return isletter(c) || isdigit(c)
}

func isint(c rune) bool {
	return isdigit(c) || ishex(c) || lower(c) == 'x'
}

// tagOptions represents a parsed struct tag.
type tagOptions struct {
	Ignore           bool
	Type             reflect.Type
	SizeOf           string
	SizeFrom         string
	Skip             int
	Order            binary.ByteOrder
	BitSize          uint8
	VariantBoolFlag  bool
	InvertedBoolFlag bool
	RootFlag         bool
	ParentFlag       bool
	DefaultFlag      bool

	IfExpr     string
	SizeExpr   string
	BitsExpr   string
	InExpr     string
	OutExpr    string
	WhileExpr  string
	SwitchExpr string
	CaseExpr   string
}

func (opts *tagOptions) parse(tag string) error {
	// Empty tag
	if len(tag) == 0 {
		return nil
	} else if tag == "-" {
		opts.Ignore = true
		return nil
	}

	tag += "\x00"

	accept := func(v string) bool {
		if strings.HasPrefix(tag, v) {
			tag = tag[len(v):]
			return true
		}
		return false
	}

	acceptIdent := func() (string, error) {
		var (
			i int
			r rune
		)
		for i, r = range tag {
			if r == ',' || r == 0 {
				break
			}
			if i == 0 && !isletter(r) || !isident(r) {
				return "", fmt.Errorf("invalid identifier character %c", r)
			}
		}
		result := tag[:i]
		tag = tag[i:]
		return result, nil
	}

	acceptInt := func() (int, error) {
		var (
			i int
			r rune
		)
		for i, r = range tag {
			if r == ',' || r == 0 {
				break
			}
			if !isint(r) {
				return 0, fmt.Errorf("invalid integer character %c", r)
			}
		}
		result := tag[:i]
		tag = tag[i:]
		d, err := strconv.ParseInt(result, 0, 64)
		return int(d), err
	}

	acceptExpr := func() (string, error) {
		stack := []byte{0}

		current := func() byte { return stack[len(stack)-1] }
		push := func(r byte) { stack = append(stack, r) }
		pop := func() { stack = stack[:len(stack)-1] }

		var i int
	expr:
		for i = 0; i < len(tag); i++ {
			switch tag[i] {
			case ',':
				if len(stack) == 1 {
					break expr
				}
			case '(':
				push(')')
			case '[':
				push(']')
			case '{':
				push('}')
			case '"', '\'':
				term := tag[i]
				i++
			lit:
				for {
					if i >= len(tag) {
						return "", errors.New("unexpected eof in literal")
					}
					switch tag[i] {
					case term:
						break lit
					case '\\':
						i++
					}
					i++
				}
			case current():
				pop()
				if len(stack) == 0 {
					break expr
				}
			default:
				if tag[i] == 0 {
					return "", errors.New("unexpected eof in expr")
				}
			}
		}
		result := tag[:i]
		tag = tag[i:]
		return result, nil
	}

	var err error
	for {
		switch {
		case accept("lsb"), accept("little"):
			opts.Order = binary.LittleEndian
		case accept("msb"), accept("big"), accept("network"):
			opts.Order = binary.BigEndian
		case accept("variantbool"):
			opts.VariantBoolFlag = true
		case accept("invertedbool"):
			opts.InvertedBoolFlag = true
		case accept("root"):
			opts.RootFlag = true
		case accept("parent"):
			opts.ParentFlag = true
		case accept("default"):
			opts.DefaultFlag = true
		case accept("sizeof="):
			if opts.SizeOf, err = acceptIdent(); err != nil {
				return fmt.Errorf("sizeof: %v", err)
			}
		case accept("sizefrom="):
			if opts.SizeFrom, err = acceptIdent(); err != nil {
				return fmt.Errorf("sizefrom: %v", err)
			}
		case accept("skip="):
			if opts.Skip, err = acceptInt(); err != nil {
				return fmt.Errorf("skip: %v", err)
			}
		case accept("if="):
			if opts.IfExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("if: %v", err)
			}
		case accept("size="):
			if opts.SizeExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("size: %v", err)
			}
		case accept("bits="):
			if opts.BitsExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("bits: %v", err)
			}
		case accept("in="):
			if opts.InExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("in: %v", err)
			}
		case accept("out="):
			if opts.OutExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("out: %v", err)
			}
		case accept("while="):
			if opts.WhileExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("while: %v", err)
			}
		case accept("switch="):
			if opts.SwitchExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("switch: %v", err)
			}
		case accept("case="):
			if opts.CaseExpr, err = acceptExpr(); err != nil {
				return fmt.Errorf("case: %v", err)
			}
		case accept("-"):
			return errors.New("extra options on ignored field")
		default:
			typeexpr, err := acceptExpr()
			if err != nil {
				return fmt.Errorf("struct type: %v", err)
			}
			parts := strings.SplitN(typeexpr, ":", 2)
			opts.Type, err = parseType(parts[0])
			if err != nil {
				return fmt.Errorf("struct type: %v", err)
			}
			if len(parts) < 2 {
				break
			}
			if !validBitType(opts.Type) {
				return fmt.Errorf("struct type bits specified on non-bitwise type %s", opts.Type)
			}
			bits, err := strconv.ParseUint(parts[1], 0, 8)
			if err != nil {
				return errors.New("struct type bits: invalid integer syntax")
			}
			opts.BitSize = uint8(bits)
			if opts.BitSize >= uint8(opts.Type.Bits()) || opts.BitSize == 0 {
				return fmt.Errorf("bit size %d out of range (%d to %d)", opts.BitSize, 1, opts.Type.Bits()-1)
			}
		}
		if accept("\x00") {
			return nil
		}
		if !accept(",") {
			return errors.New("tag: expected comma")
		}
	}
}

// mustParseTag calls ParseTag but panics if there is an error, to help make
// sure programming errors surface quickly.
func mustParseTag(tag string) tagOptions {
	opt, err := parseTag(tag)
	if err != nil {
		panic(err)
	}
	return opt
}

// parseTag parses a struct tag into a TagOptions structure.
func parseTag(tag string) (tagOptions, error) {
	opts := tagOptions{}
	if err := opts.parse(tag); err != nil {
		return tagOptions{}, err
	}
	return opts, nil
}
