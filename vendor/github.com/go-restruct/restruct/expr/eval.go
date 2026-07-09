package expr

import (
	"fmt"
)

// EvalProgram returns the result of executing the program with the given resolver.
func EvalProgram(resolver Resolver, program *Program) (v interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			if rerr, ok := r.(error); ok {
				err = rerr
			} else {
				panic(r)
			}
		}
	}()

	v = evalnode(resolver, program.root).RawValue()
	return
}

// Eval returns the result of evaluating the provided expression.
func Eval(resolver Resolver, expr string) (interface{}, error) {
	return EvalProgram(resolver, ParseString(expr))
}

func evalnode(resolver Resolver, node node) Value {
	switch n := node.(type) {
	case identnode:
		v := resolver.Resolve(n.ident)
		if v == nil {
			panic(fmt.Errorf("unresolved name %s", n.ident))
		}
		return v
	case intnode:
		if n.sign {
			return literalintval(n.ival)
		}
		return literaluintval(n.uval)
	case floatnode:
		return literalfloatval(n.fval)
	case boolnode:
		return literalboolval(n.val)
	case strnode:
		return literalstrval(n.val)
	case runenode:
		return literalintval(int64(n.val))
	case nilnode:
		return literalnilval()
	case unaryexpr:
		return evalunary(resolver, n)
	case binaryexpr:
		return evalbinary(resolver, n)
	case ternaryexpr:
		return evalternary(resolver, n)
	default:
		panic("invalid node")
	}
}

func evalunary(resolver Resolver, node unaryexpr) Value {
	n := evalnode(resolver, node.n)
	switch node.op {
	case unaryplus:
		return n
	case unarynegate:
		return n.Negate()
	case unarynot:
		return n.Not()
	case unarybitnot:
		return n.BitNot()
	case unaryderef:
		return n.Deref()
	case unaryref:
		return n.Ref()
	default:
		panic("invalid unary expression")
	}
}

func flattengroup(n node) []node {
	if n, ok := n.(binaryexpr); ok {
		if n.op == binarygroup {
			return append(flattengroup(n.a), flattengroup(n.b)...)
		}
	}
	return []node{n}
}

func evalbinary(resolver Resolver, node binaryexpr) Value {
	a := evalnode(resolver, node.a)
	switch node.op {
	case binarymember:
		if id, ok := node.b.(identnode); ok {
			return a.Dot(id.ident)
		}
		panic(fmt.Errorf("expected ident node, got %T", node.b))
	case binarycall:
		in := []Value{}
		for _, n := range flattengroup(node.b) {
			in = append(in, evalnode(resolver, n))
		}
		return a.Call(in)
	case binarygroup:
		return evalnode(resolver, node.b)
	}

	b := evalnode(resolver, node.b)
	switch node.op {
	case binarylogicalor:
		return a.LogicalOr(b)
	case binarylogicaland:
		return a.LogicalAnd(b)
	case binaryequal:
		return a.Equal(b)
	case binarynotequal:
		return a.NotEqual(b)
	case binarylesser:
		return a.Lesser(b)
	case binarylesserequal:
		return a.LesserEqual(b)
	case binarygreater:
		return a.Greater(b)
	case binarygreaterequal:
		return a.GreaterEqual(b)
	case binaryadd:
		return a.Add(b)
	case binarysub:
		return a.Sub(b)
	case binaryor:
		return a.Or(b)
	case binaryxor:
		return a.Xor(b)
	case binarymul:
		return a.Mul(b)
	case binarydiv:
		return a.Div(b)
	case binaryrem:
		return a.Rem(b)
	case binarylsh:
		return a.Lsh(b)
	case binaryrsh:
		return a.Rsh(b)
	case binaryand:
		return a.And(b)
	case binaryandnot:
		return a.AndNot(b)
	case binarysubscript:
		return a.Index(b)
	default:
		panic("invalid binary expression")
	}
}

func evalternary(resolver Resolver, node ternaryexpr) Value {
	a := evalnode(resolver, node.a).Value().Interface()
	cond, ok := a.(bool)
	if !ok {
		panic(fmt.Errorf("unexpected type %T for ternary", cond))
	}
	if cond {
		return evalnode(resolver, node.b)
	}
	return evalnode(resolver, node.c)
}
