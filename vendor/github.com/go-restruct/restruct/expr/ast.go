package expr

import (
	"fmt"
	"strconv"
)

type node interface {
	source() string
}

type unaryop int

const (
	unaryplus unaryop = iota
	unarynegate
	unarynot
	unarybitnot
	unaryderef
	unaryref
)

type binaryop int

const (
	binarylogicalor binaryop = iota
	binarylogicaland
	binaryequal
	binarynotequal
	binarylesser
	binarylesserequal
	binarygreater
	binarygreaterequal
	binaryadd
	binarysub
	binaryor
	binaryxor
	binarymul
	binarydiv
	binaryrem
	binarylsh
	binaryrsh
	binaryand
	binaryandnot
	binarymember
	binarycall
	binarysubscript
	binarygroup
)

// Identifier node.
type identnode struct {
	pos   int
	ident string
}

func newidentnode(t token) identnode {
	return identnode{t.pos, t.sval}
}

func (n identnode) source() string {
	return n.ident
}

// Integer literal node.
type intnode struct {
	pos  int
	uval uint64
	ival int64
	sign bool
}

func newintnode(t token) intnode {
	return intnode{pos: t.pos, uval: t.uval, ival: t.ival, sign: t.sign}
}

func (n intnode) source() string {
	if n.sign {
		return strconv.FormatInt(n.ival, 10)
	}
	return strconv.FormatUint(n.uval, 10)
}

// Float literal node.
type floatnode struct {
	pos  int
	fval float64
}

func newfloatnode(t token) floatnode {
	return floatnode{pos: t.pos, fval: t.fval}
}

func (n floatnode) source() string {
	return strconv.FormatFloat(n.fval, 'f', -1, 64)
}

// Bool literal node.
type boolnode struct {
	pos int
	val bool
}

func newboolnode(t token) boolnode {
	return boolnode{t.pos, t.bval}
}

func (n boolnode) source() string {
	if n.val {
		return "true"
	}
	return "false"
}

// String literal node.
type strnode struct {
	pos int
	val string
}

func newstrnode(t token) strnode {
	return strnode{t.pos, t.sval}
}

func (n strnode) source() string {
	return fmt.Sprintf("%q", n.val)
}

// Rune literal node.
type runenode struct {
	pos int
	val rune
}

func newrunenode(t token) runenode {
	return runenode{t.pos, rune(t.ival)}
}

func (n runenode) source() string {
	return fmt.Sprintf("%q", n.val)
}

// Nil node.
type nilnode struct {
	pos int
}

func newnilnode(t token) nilnode {
	return nilnode{t.pos}
}

func (nilnode) source() string {
	return "nil"
}

// Unary expression node.
type unaryexpr struct {
	op unaryop
	n  node
}

func (n unaryexpr) source() string {
	operand := n.n.source()
	switch n.op {
	case unaryplus:
		return "+" + operand
	case unarynegate:
		return "-" + operand
	case unarynot:
		return "!" + operand
	case unarybitnot:
		return "^" + operand
	case unaryderef:
		return "*" + operand
	case unaryref:
		return "&" + operand
	}
	panic("invalid unary expr?")
}

// Binary expression node.
type binaryexpr struct {
	op   binaryop
	a, b node
}

func (n binaryexpr) source() string {
	a, b := n.a.source(), n.b.source()
	switch n.op {
	case binarylogicalor:
		return a + " || " + b
	case binarylogicaland:
		return a + " && " + b
	case binaryequal:
		return a + " == " + b
	case binarynotequal:
		return a + " != " + b
	case binarylesser:
		return a + " < " + b
	case binarylesserequal:
		return a + " <= " + b
	case binarygreater:
		return a + " > " + b
	case binarygreaterequal:
		return a + " >= " + b
	case binaryadd:
		return a + " + " + b
	case binarysub:
		return a + " - " + b
	case binaryor:
		return a + " | " + b
	case binaryxor:
		return a + " ^ " + b
	case binarymul:
		return a + " * " + b
	case binarydiv:
		return a + " / " + b
	case binaryrem:
		return a + " % " + b
	case binarylsh:
		return a + " << " + b
	case binaryrsh:
		return a + " >> " + b
	case binaryand:
		return a + " & " + b
	case binaryandnot:
		return a + " &^ " + b
	case binarymember:
		return a + "." + b
	case binarycall:
		return a + "(" + b + ")"
	case binarysubscript:
		return a + "[" + b + "]"
	case binarygroup:
		return a + ", " + b
	}
	panic("invalid binary expr?")
}

// Ternary expression node.
type ternaryexpr struct {
	a, b, c node
}

func (n ternaryexpr) source() string {
	return n.a.source() + " ? " + n.b.source() + " : " + n.c.source()
}
