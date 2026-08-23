package restruct

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
)

// typeMap maps identifiers to reflect.Types.
var typeMap = map[string]reflect.Type{
	"bool": reflect.TypeOf(bool(false)),

	"uint8":  reflect.TypeOf(uint8(0)),
	"uint16": reflect.TypeOf(uint16(0)),
	"uint32": reflect.TypeOf(uint32(0)),
	"uint64": reflect.TypeOf(uint64(0)),

	"int8":  reflect.TypeOf(int8(0)),
	"int16": reflect.TypeOf(int16(0)),
	"int32": reflect.TypeOf(int32(0)),
	"int64": reflect.TypeOf(int64(0)),

	"float32": reflect.TypeOf(float32(0)),
	"float64": reflect.TypeOf(float64(0)),

	"complex64":  reflect.TypeOf(complex64(0)),
	"complex128": reflect.TypeOf(complex128(0)),

	"byte": reflect.TypeOf(uint8(0)),
	"rune": reflect.TypeOf(int32(0)),

	"uint":    reflect.TypeOf(uint(0)),
	"int":     reflect.TypeOf(int(0)),
	"uintptr": reflect.TypeOf(uintptr(0)),
	"string":  reflect.SliceOf(reflect.TypeOf(uint8(0))),
}

// typeOfExpr gets a type corresponding to an expression.
func typeOfExpr(expr ast.Expr) (reflect.Type, error) {
	switch expr := expr.(type) {
	default:
		return nil, fmt.Errorf("unexpected expression: %T", expr)
	case *ast.ArrayType:
		switch expr.Len {
		case ast.Expr(nil):
			// Slice
			sub, err := typeOfExpr(expr.Elt)
			if err != nil {
				return nil, err
			}
			return reflect.SliceOf(sub), nil
		default:
			// Parse length expression
			lexpr, ok := expr.Len.(*ast.BasicLit)
			if !ok {
				return nil, fmt.Errorf("invalid array size expression")
			}
			if lexpr.Kind != token.INT {
				return nil, fmt.Errorf("invalid array size type")
			}
			len, err := strconv.Atoi(lexpr.Value)
			if err != nil {
				return nil, err
			}

			// Parse elem type expression
			sub, err := typeOfExpr(expr.Elt)
			if err != nil {
				return nil, err
			}
			return reflect.ArrayOf(len, sub), nil
		}
	case *ast.Ident:
		// Primitive types
		typ, ok := typeMap[expr.Name]
		if !ok {
			return nil, fmt.Errorf("unknown type %s", expr.Name)
		}
		return typ, nil
	case *ast.StarExpr:
		// Pointer
		sub, err := typeOfExpr(expr.X)
		if err != nil {
			return nil, err
		}
		return reflect.PtrTo(sub), nil
	case *ast.ChanType:
		return nil, fmt.Errorf("channel type not allowed")
	case *ast.MapType:
		return nil, fmt.Errorf("map type not allowed")
	}
}

// parseType parses a Golang type string and returns a reflect.Type.
func parseType(typ string) (reflect.Type, error) {
	expr, err := parser.ParseExpr(typ)
	if err != nil {
		return nil, errors.Wrap(err, "parsing error")
	}

	return typeOfExpr(expr)
}
