package expr

import (
	"bytes"
	"fmt"
	"io"
)

// Program represents a parsed expression.
type Program struct {
	root node
}

// Parse parses an expression into a program.
func Parse(r io.RuneScanner) *Program {
	return &Program{newparser(newscanner(r)).parse()}
}

// ParseString parses an expression from a string.
func ParseString(s string) *Program {
	return Parse(bytes.NewBufferString(s))
}

type parser struct {
	s *scanner

	a bool
	t token
}

func newparser(s *scanner) *parser {
	return &parser{s: s}
}

func (p *parser) readtoken() *token {
	if !p.a {
		p.a = true
		p.t = p.s.scan()
	}
	return &p.t
}

func (p *parser) consume() {
	p.a = false
}

func (p *parser) accept(k tokenkind) bool {
	if p.readtoken().kind == k {
		p.consume()
		return true
	}
	return false
}

func (p *parser) expect(k tokenkind) {
	if p.readtoken().kind != k {
		panic(fmt.Errorf("expected %s token, got %s", k, p.t.kind))
	}
	p.consume()
}

// This parser is strongly based on byuu's modified recursive-descent algorithm
// (particularly the 'depth' parameter.)
// https://github.com/byuu/bsnes/blob/master/nall/string/eval/parser.hpp
func (p *parser) parseexpr(depth int) node {
	var n node

	unary := func(op unaryop, depth int) {
		n = unaryexpr{op: op, n: p.parseexpr(depth)}
	}

	binary := func(op binaryop, depth int) {
		if n == nil {
			panic("unexpected binary op")
		}
		n = binaryexpr{op: op, a: n, b: p.parseexpr(depth)}
	}

	ternary := func(depth int) {
		t := ternaryexpr{}
		t.a = n
		t.b = p.parseexpr(depth)
		p.expect(colontoken)
		t.c = p.parseexpr(depth)
		n = t
	}

	switch {
	case p.accept(identtoken):
		n = newidentnode(p.t)
	case p.accept(inttoken):
		n = newintnode(p.t)
	case p.accept(floattoken):
		n = newfloatnode(p.t)
	case p.accept(booltoken):
		n = newboolnode(p.t)
	case p.accept(strtoken):
		n = newstrnode(p.t)
	case p.accept(runetoken):
		n = newrunenode(p.t)
	case p.accept(nilkeyword):
		n = newnilnode(p.t)
	case p.accept(leftparentoken):
		n = p.parseexpr(1)
	default:
	}

	for {
		if depth >= 8 {
			break
		}
		if n != nil && p.accept(periodtoken) {
			binary(binarymember, 8)
			continue
		}
		if n != nil && p.accept(leftparentoken) {
			binary(binarycall, 1)
			continue
		}
		if n != nil && p.accept(leftbrackettoken) {
			binary(binarysubscript, 1)
			continue
		}
		if n == nil && p.accept(addtoken) {
			unary(unaryplus, 7)
		}
		if n == nil && p.accept(subtoken) {
			unary(unarynegate, 7)
		}
		if n == nil && p.accept(nottoken) {
			unary(unarynot, 7)
		}
		if n == nil && p.accept(xortoken) {
			unary(unarybitnot, 7)
		}
		if n == nil && p.accept(multoken) {
			unary(unaryderef, 7)
		}
		if n == nil && p.accept(andtoken) {
			unary(unaryref, 7)
		}
		if depth >= 7 {
			break
		}
		if p.accept(multoken) {
			binary(binarymul, 7)
			continue
		}
		if p.accept(quotoken) {
			binary(binarydiv, 7)
			continue
		}
		if p.accept(remtoken) {
			binary(binaryrem, 7)
			continue
		}
		if p.accept(shltoken) {
			binary(binarylsh, 7)
			continue
		}
		if p.accept(shrtoken) {
			binary(binaryrsh, 7)
			continue
		}
		if p.accept(andtoken) {
			binary(binaryand, 7)
			continue
		}
		if depth >= 6 {
			break
		}
		if p.accept(addtoken) {
			binary(binaryadd, 6)
			continue
		}
		if p.accept(subtoken) {
			binary(binarysub, 6)
			continue
		}
		if p.accept(ortoken) {
			binary(binaryor, 6)
			continue
		}
		if p.accept(xortoken) {
			binary(binaryxor, 6)
			continue
		}
		if depth >= 5 {
			break
		}
		if p.accept(equaltoken) {
			binary(binaryequal, 5)
			continue
		}
		if p.accept(notequaltoken) {
			binary(binarynotequal, 5)
			continue
		}
		if p.accept(lessertoken) {
			binary(binarylesser, 5)
			continue
		}
		if p.accept(lesserequaltoken) {
			binary(binarylesserequal, 5)
			continue
		}
		if p.accept(greatertoken) {
			binary(binarygreater, 5)
			continue
		}
		if p.accept(greaterequaltoken) {
			binary(binarygreaterequal, 5)
			continue
		}
		if depth >= 4 {
			break
		}
		if p.accept(logicalandtoken) {
			binary(binarylogicaland, 4)
			continue
		}
		if depth >= 3 {
			break
		}
		if p.accept(logicalortoken) {
			binary(binarylogicalor, 3)
			continue
		}
		if p.accept(ternarytoken) {
			ternary(3)
			continue
		}
		if depth >= 2 {
			break
		}
		if p.accept(commatoken) {
			binary(binarygroup, 2)
			continue
		}
		if depth >= 1 && (p.accept(rightparentoken) || p.accept(rightbrackettoken)) {
			break
		}
		p.expect(eoftoken)
		break
	}
	return n
}

func (p *parser) parse() node {
	return p.parseexpr(0)
}
