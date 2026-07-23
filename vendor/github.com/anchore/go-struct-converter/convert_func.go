package converter

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
)

type FuncChain interface {
	AddConverter(converter ...any) FuncChain
	AutoPackageConverter(fromPkg, toPkg any) FuncChain
	AllowImplicit() FuncChain
	Convert(from any, to any) error
}

type funcChain struct {
	allowImplicitConversion bool
	funcs                   map[reflect.Type]map[reflect.Type]func(from reflect.Value, to reflect.Value) error
}

func NewFuncChain(converters ...any) FuncChain {
	out := funcChain{
		funcs: map[reflect.Type]map[reflect.Type]func(from reflect.Value, to reflect.Value) error{},
	}
	return out.AddConverter(converters...)
}

func (c *funcChain) AllowImplicit() FuncChain {
	c.allowImplicitConversion = true
	return c
}

func (c *funcChain) AutoPackageConverter(fromPkg, toPkg any) FuncChain {
	fromTypes := map[string]reflect.Type{}
	toTypes := map[string]reflect.Type{}

	fromName := pkgName(fromPkg)
	toName := pkgName(toPkg)

	if fromName == "" || toName == "" {
		panic("invalid auto package type; should be struct")
	}

	for t := range listAllBaseTypes() {
		if t.PkgPath() == fromPkg {
			fromTypes[t.Name()] = t
		}
		if t.PkgPath() == toPkg {
			toTypes[t.Name()] = t
		}
	}

	for name, fromT := range fromTypes {
		toT, ok := toTypes[name]
		if !ok {
			continue
		}

		// this does nothing other than inform the types
		c.AddConvertFunc(fromT, toT, func(_ reflect.Value, _ reflect.Value) error {
			return nil
		})
	}

	return c
}

func (c *funcChain) Convert(from any, to any) error {
	fromValue := reflect.ValueOf(from)
	fromType := fromValue.Type()
	baseFromType := baseType(fromType)

	toValue := reflect.ValueOf(to)
	toType := toValue.Type()
	baseToType := baseType(toType)

	// build the shortest path between types
	chain := c.shortestChain(baseFromType, baseToType)

	// no explicit conversions
	if len(chain) == 0 {
		return fmt.Errorf("no conversion path found from %s to %s", typeName(baseFromType), typeName(baseToType))
	}

	cnv := conversion{
		chain: c,
	}

	// iterate, creating any intermediary structs for the migration
	last := fromValue
	for i, step := range chain {
		var next reflect.Value
		if i == len(chain)-1 {
			next = toValue
		} else {
			next = reflect.New(step.targetType)
		}

		cnv.convert(last, next)
		last = next
	}

	return errors.Join(cnv.errors...)
}

func (c *funcChain) AddConverter(converters ...any) FuncChain {
	for _, converter := range converters {
		c.addConverter(converter)
	}
	return c
}

func (c *funcChain) addConverter(converter any) {
	convertFunc := reflect.ValueOf(converter)
	convertFuncType := convertFunc.Type()
	if validationError := validateConvertFunc(convertFuncType); validationError != nil {
		panic(fmt.Errorf(`converter must be a function of one of the following forms:
			func(from *Type1, to *Type2)
			func(from *Type1, to *Type2) error
			func(chain %v, from *Type1, to *Type2)
			func(chain %v, from *Type1, to *Type2) error

			got: %+v
			err: %v
		`, chainType, chainType, convertFuncType, validationError))
	}

	// seems to be a valid function, create a handler function for it

	returnsError := convertFuncType.NumOut() > 0

	hasChainParam := false
	fromType := convertFuncType.In(0)
	toType := convertFuncType.In(1)

	if convertFuncType.NumIn() > 2 {
		hasChainParam = true
		fromType = convertFuncType.In(1)
		toType = convertFuncType.In(2)
	}

	c.AddConvertFunc(fromType, toType, func(from reflect.Value, to reflect.Value) error {
		// setup matching args, from and to should already be set up properly
		var args []reflect.Value
		if hasChainParam {
			args = []reflect.Value{reflect.ValueOf(c), from, to}
		} else {
			args = []reflect.Value{from, to}
		}

		// invoke the function
		out := convertFunc.Call(args)

		// return errors if the function does
		if returnsError && !out[0].IsNil() {
			return out[0].Interface().(error)
		}
		return nil
	})
}

func (c *funcChain) AddConvertFunc(fromType, toType reflect.Type, fn func(from reflect.Value, to reflect.Value) error) {
	baseFromType := baseType(fromType)
	baseToType := baseType(toType)

	convertFuncs := c.funcs[baseFromType]
	if convertFuncs == nil {
		convertFuncs = map[reflect.Type]func(from reflect.Value, to reflect.Value) error{}
		c.funcs[baseFromType] = convertFuncs
	}

	if convertFuncs[baseToType] != nil {
		panic(fmt.Errorf("convert from: %s -> %s defined multiple times; %+v", typeName(baseFromType), typeName(baseToType), reflect.TypeOf(convertFuncs[baseToType])))
	}

	convertFuncs[baseToType] = fn
}

func (c *funcChain) shortestChain(fromType reflect.Type, targetType reflect.Type, visited ...reflect.Type) []reflectConvertStep {
	var shortest []reflectConvertStep
	for toType, convertFunc := range c.funcs[fromType] {
		if slices.Contains(visited, toType) {
			continue
		}
		if toType == targetType {
			return []reflectConvertStep{{toType, convertFunc}}
		}
		chain := c.shortestChain(toType, targetType, append(visited, fromType)...)
		if chain == nil {
			continue
		}
		// this is a viable conversion chain, use it if it's shorter or we haven't found any yet
		chain = append([]reflectConvertStep{{toType, convertFunc}}, chain...)
		if shortest == nil || len(chain) < len(shortest) {
			shortest = chain
		}
	}
	// no explicit conversions, try a direct conversion
	if len(shortest) == 0 && c.allowImplicitConversion {
		return []reflectConvertStep{{fromType, func(_ reflect.Value, _ reflect.Value) error {
			return nil
		}}}
	}
	return shortest
}

var chainType = reflect.TypeOf((*FuncChain)(nil)).Elem()
var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func typeName(t reflect.Type) string {
	return fmt.Sprintf("<%s>.%s", t.PkgPath(), t.Name())
}

func validateConvertFunc(t reflect.Type) error {
	if t.Kind() != reflect.Func {
		return fmt.Errorf("not a function")
	}
	// need to have 2 or 3 args, optionally with funcChain as the first one
	if t.NumIn() < 2 || t.NumIn() > 3 {
		return fmt.Errorf("must have 2 or 3 arguments")
	}
	fromType := t.In(0)
	toType := t.In(1)
	if t.NumIn() > 2 {
		if t.In(0) != chainType {
			return fmt.Errorf("when using 3 arguments, %+v must the first", chainType)
		}
		fromType = t.In(1)
		toType = t.In(2)
	} else if t.In(0) == chainType {
		return fmt.Errorf("if %+v is the first argument, there must be 2 more arguments to convert", chainType)
	}

	// it doesn't make sense to convert from a type to the same type
	if baseType(fromType) == baseType(toType) {
		return fmt.Errorf("convert should be between different types")
	}
	// toType must be a pointer, which will be provided by the convert function
	if !isPtr(toType) {
		return fmt.Errorf("second convert argument, the target/destination must be a pointer; got: %+v", toType)
	}
	// return type is either error or nothing
	if t.NumOut() > 1 {
		return fmt.Errorf("too many return values, must return error or have no return value")
	}
	if t.NumOut() > 0 && !t.Out(0).Implements(errorInterface) {
		return fmt.Errorf("must return error or have no return value")
	}

	return nil
}

func pkgName(pkg any) string {
	switch p := pkg.(type) {
	case string:
		return p
	case reflect.Type:
		return p.PkgPath()
	}
	return baseType(reflect.TypeOf(pkg)).PkgPath()
}

func baseType(t reflect.Type) reflect.Type {
	for isPtr(t) {
		t = t.Elem()
	}
	return t
}

type reflectConvertFunc func(from reflect.Value, to reflect.Value) error

type reflectConvertStep struct {
	targetType  reflect.Type
	convertFunc reflectConvertFunc
}
