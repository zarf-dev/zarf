package fangs

import (
	"fmt"
	"reflect"

	"github.com/spf13/pflag"

	"github.com/anchore/go-logger"
)

// FlagAdder interface can be implemented by structs in order to add flags when AddFlags is called
type FlagAdder interface {
	AddFlags(flags FlagSet)
}

// AddFlags traverses the object graphs from the structs provided and calls all AddFlags methods implemented on them
func AddFlags(log logger.Logger, flags *pflag.FlagSet, structs ...any) {
	flagSet := NewPFlagSet(log, flags)
	for _, o := range structs {
		addFlags(log, flagSet, o)
	}
}

func addFlags(log logger.Logger, flags FlagSet, o any) {
	v := reflect.ValueOf(o)
	if !isPtr(v.Type()) {
		panic(fmt.Sprintf("AddFlags must be called with pointers, got: %#v", o))
	}

	invokeAddFlags(log, flags, o)

	v, t := base(v)

	if isStruct(t) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !includeField(f) {
				continue
			}
			v := v.Field(i)

			if isPtr(v.Type()) {
				// check if this is a pointer to a struct, if so, we need to initialize it
				kind := v.Type().Elem().Kind()
				if v.IsNil() && kind == reflect.Struct {
					newV := reflect.New(v.Type().Elem())
					if v.CanSet() {
						v.Set(newV)
					}
				}
			} else {
				v = v.Addr()
			}

			if !v.CanInterface() {
				continue
			}

			addFlags(log, flags, v.Interface())
		}
	}
}

func invokeAddFlags(_ logger.Logger, flags FlagSet, o any) {
	// defer func() {
	//	// we may need to handle embedded structs having AddFlags methods called,
	//	// potentially adding flags with existing names. currently the isPromotedMethod
	//  // function works, but it is fairly brittle as there is no way through standard
	//  // go reflection to ascertain this information
	//	if err := recover(); err != nil {
	//		if log == nil {
	//			panic(err)
	//		}
	//		log.Debugf("got error while invoking AddFlags: %v", err)
	//	}
	// }()

	if o, ok := o.(FlagAdder); ok && !isPromotedMethod(o, "AddFlags") {
		o.AddFlags(flags)
	}
}
