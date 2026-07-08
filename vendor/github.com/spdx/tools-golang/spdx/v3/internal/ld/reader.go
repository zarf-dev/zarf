package ld

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/piprate/json-gold/ld"
)

type mapReader struct {
	ctx       *context
	errs      []error
	logOut    io.Writer                // logOut if set will result in lots of log messages about the processing
	link      bool                     // link indicates this is a second pass to link previously created instances
	instances map[string]reflect.Value // instances holds id -> initialized instances
}

func (c *mapReader) FromMaps(values map[string]any) ([]any, error) {
	proc := ld.NewJsonLdProcessor()
	opts := ld.NewJsonLdOptions("")
	opts.ProcessingMode = ld.JsonLd_1_1_Frame
	opts.DocumentLoader = offlineDocumentLoader{ctx: c.ctx}
	//opts.Embed=            EmbedLast,
	//opts.Explicit=         false,
	//opts.RequireAll=       true,
	opts.FrameDefault = true

	//opts.OmitDefault = true
	//opts.OmitGraph=        false,

	//opts.UseRdfType = false
	//opts.UseNativeTypes=   false,
	//opts.ProduceGeneralizedRdf: false,
	//opts.InputFormat=      "",
	//opts.Format=           "",
	//opts.Algorithm=        AlgorithmURGNA2012,
	//opts.UseNamespaces=    false,
	//opts.OutputForm=       "",
	//opts.SafeMode = true
	expanded, err := proc.Expand(values, opts)
	c.log(nil, "expanded graph: %v", expanded)
	if err != nil {
		return nil, err
	}
	return c.FromSlice(expanded)
}

func (c *mapReader) FromSlice(expanded []any) ([]any, error) {
	c.instances = map[string]reflect.Value{}

	path := []string{JsonGraphProp}

	// one pass to create all the instances
	_ = c.readSlice(path, anyType, expanded)

	// second pass captures all errors, and links all instances which were created previously
	c.link = true

	// the ld expansion above reads @graph and returns a fully expanded graph
	var out []any
	for _, value := range c.readSlice(path, anyType, expanded) {
		out = append(out, value.Interface())
	}

	return out, errors.Join(c.errs...)
}

// readSlice takes a fully expanded list of nodes and returns the go struct representations
func (c *mapReader) readSlice(path []string, targetType reflect.Type, values []any) []reflect.Value {
	var out []reflect.Value
	for i, node := range values {
		got := c.getNode(append(path, strconv.Itoa(i)), targetType, node)
		// only valid objects which can be returned via .Interface() are allowed
		if got.IsValid() && got.CanInterface() {
			out = append(out, got)
		}
	}
	return out
}

// readNode gets object instances and primitives based on a node value, creating new instances as needed, we only expect
// to get a map[string]any as input here
func (c *mapReader) getNode(path []string, targetType reflect.Type, incoming any) reflect.Value {
	values, ok := incoming.(map[string]any)
	if !ok {
		c.err(path, "expected object, got: %v", incoming)
		return emptyValue
	}
	// not a primitive expected is some sort of struct (pointer, interface, etc.)
	id, _ := values[JsonIdProp].(string)
	typeIRI := singleValue[string](values[JsonTypeProp])
	value, _ := values[JsonValueProp]

	// type & value is an external IRI
	if typeIRI != "" && value != nil {
		cnv := iriToConverter[typeIRI]
		if cnv != nil {
			if cnv.Deserialize != nil {
				value = cnv.Deserialize(value)
			}
			if value != nil {
				v := reflect.ValueOf(value)
				// can directly assign, value is good
				if v.Type().AssignableTo(cnv.Type) {
					return v
				}
				// can convert, still good
				if v.CanConvert(cnv.Type) {
					return v.Convert(cnv.Type)
				}
				c.err(path, "invalid value: %v", value)
			}
		}

		return emptyValue
	}

	tc := c.ctx.iriToType[typeIRI]
	if tc == nil {
		if id != "" {
			// first look up any known named individuals, we do not populate these
			if v, ok := c.ctx.iriToInstance[id]; ok {
				return v
			}

			// if there isn't a named individual, and we don't have a type, it may be a reference in the same document,
			// but it may not have been created yet, here we just want to return
			// we may have created this instance already
			instance, ok := c.instances[id]
			if ok {
				return instance
			}

			if c.link { // only need external IRI references on the second pass
				// if we have no type and don't have an instance created, return an external IRI
				return c.externalIRI(path, targetType, id)
			}
		}
		return emptyValue
	}

	//	// TODO support setting map values
	// 	if typ.Kind() != reflect.Struct {
	//		c.err(path, "unable to set struct properties on non-struct type: %s", typeName(instance.Type()))
	//		return
	//	}

	return c.readObject(path, targetType, id, tc, values)
}

func (c *mapReader) readObject(path []string, targetType reflect.Type, id string, tc *typeContext, incoming map[string]any) reflect.Value {
	// we created this instance already during the first pass, look up that instance
	instance, ok := c.instances[id]
	if !ok {
		instance, ok = c.ctx.iriToInstance[id]
		if ok {
			return instance
		}
		// if not, create it now
		instance = reflect.New(baseType(tc.typ)) // New(T) returns *T
		if id != "" {
			// only set instance references when an ID is provided
			c.instances[id] = instance
		}
	}
	c.setStructFields(path, instance, id, tc, incoming)
	return instance
}

func (c *mapReader) setStructFields(path []string, instance reflect.Value, id string, tc *typeContext, incoming map[string]any) {
	typ := instance.Type()
	if typ.Kind() == reflect.Pointer {
		instance = instance.Elem()
		typ = instance.Type()
	}

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if skipField(f) {
			continue
		}

		fieldVal := instance.Field(i)
		c.log(path, "readObject  %v field %v", typeName(typ), f.Name)

		// embedded struct is how inheritance is handled, so recursively call this function to set all struct values
		if f.Anonymous {
			c.setStructFields(append(path, f.Name), fieldVal, id, tc, incoming)
		}

		propIRI := f.Tag.Get(GoIriTagName)
		if propIRI == "" {
			continue
		}

		if propIRI == JsonIdProp {
			// don't set blank node IDs, these will be regenerated on output
			if id != "" && !isBlankNodeID(id) {
				fieldVal.SetString(id)
			}
			continue
		}

		incomingVal, ok := incoming[propIRI]
		if !ok {
			continue
		}

		c.setFieldValue(append(path, f.Name), fieldVal, incomingVal)
	}
}

func (c *mapReader) setFieldValue(path []string, targetValue reflect.Value, incoming any) {
	incomingValues, ok := incoming.([]any)
	if !ok {
		c.err(path, "expected []any for property value, got: %v", incomingValues)
		return
	}

	targetType := targetValue.Type()
	if targetType.Kind() == reflect.Slice {
		c.setSliceValue(path, targetValue, incomingValues)
		return
	}

	// values are stored as a slice, even single values
	values := c.readSlice(path, targetType, incomingValues)

	// no valid values
	if len(values) == 0 {
		return
	}

	// any values returned from readSlice should be valid
	value := values[0]
	typ := value.Type()
	if typ.AssignableTo(targetType) {
		targetValue.Set(value)
	} else if typ.ConvertibleTo(targetType) {
		targetValue.Set(value.Convert(targetType))
	} else {
		c.err(path, "unable to set value expected: %s to: %s, dropping: %v", typeName(targetType), typeName(typ), incoming)
	}
}

func (c *mapReader) setSliceValue(path []string, targetValue reflect.Value, incoming []any) {
	sliceType := targetValue.Type()
	if sliceType.Kind() != reflect.Slice {
		panic("expected slice")
	}
	sz := len(incoming)
	if sz > 0 {
		elemType := sliceType.Elem()
		newSlice := reflect.MakeSlice(sliceType, 0, sz)
		values := c.readSlice(path, elemType, incoming)
		for i, value := range values {
			if value.Type().AssignableTo(elemType) {
				newSlice = reflect.Append(newSlice, value)
			} else if value.CanConvert(elemType) {
				newSlice = reflect.Append(newSlice, value)
			} else {
				c.err(append(path, strconv.Itoa(i)), "unable to convert value type: %s to: %s: %v", typeName(value.Type()), typeName(elemType), value)
			}
		}
		targetValue.Set(newSlice)
	}
}

//func (c *mapReader) findExternalReferenceType(expectedType reflect.Type) (reflect.Type, bool) {
//	tc := c.ctx.typeToContext[expectedType]
//	if tc != nil {
//		return tc.typ, true
//	}
//	bestMatch := anyType
//	for t := range c.ctx.typeToContext {
//		if t.Kind() != reflect.Struct {
//			continue
//		}
//		// the type with the fewest fields assignable to the target is a good candidate to be an abstract type
//		if reflect.PointerTo(t).AssignableTo(expectedType) && (bestMatch == anyType || bestMatch.NumField() > t.NumField()) {
//			bestMatch = t
//		}
//	}
//	if bestMatch != anyType {
//		c.ctx.typeToContext[expectedType] = &typeContext{
//			typ: bestMatch,
//		}
//		return bestMatch, true
//	}
//	return anyType, false
//}

type contextMap map[string]*serializationContext

func (c contextMap) getPrefix(ctx *serializationContext) string {
	for pfx, sc := range c {
		if sc == ctx {
			return pfx
		}
	}
	return ""
}

func (c *mapReader) getContextMap(currentContext contextMap, values map[string]any) (contextMap, error) {
	ctx := values[JsonContextProp]
	if ctx == nil {
		if currentContext == nil {
			return nil, fmt.Errorf("unable to find " + JsonContextProp)
		}
		return currentContext, nil
	}
	// TODO support named contexts, e.g.
	//namedContexts, _ := ctx.(map[string]any)

	context, _ := ctx.(string)
	sc := c.ctx.contextMap[context]
	if sc == nil {
		return nil, fmt.Errorf("unknown %s: '%s' must be in %v", JsonContextProp, context, maps.Keys(c.ctx.contextMap))
	}
	return merge(currentContext, contextMap{
		"": sc,
	}), nil
}

func (c *mapReader) externalIRI(path []string, targetType reflect.Type, id string) reflect.Value {
	for typ, f := range c.ctx.typeToExternalIriFunc {
		if typ.AssignableTo(targetType) {
			return f(id)
		}
	}
	c.err(path, "unable to find viable external IRI for: %s for ID: %s", typeName(targetType), id)
	return emptyValue
}

func (c *mapReader) err(path []string, format string, args ...any) {
	if c.link { // only capture errors during second pass
		c.errs = append(c.errs, fmt.Errorf("[%s] "+format, append([]any{strings.Join(path, "/")}, args...)...))
	}
}

func (c *mapReader) log(path []string, format string, args ...any) {
	if c.logOut != nil {
		caller := ""
		pc, _, _, ok := runtime.Caller(1)
		if ok {
			details := runtime.FuncForPC(pc)
			file, line := details.FileLine(pc)
			caller = details.Name() + " (" + file + ":" + strconv.Itoa(line) + "): "
		}
		_, _ = fmt.Fprintf(c.logOut, "[%s] "+caller+format+"\n", append([]any{strings.Join(path, "/")}, args...)...)
	}
}

func convertTo(v reflect.Value, typ reflect.Type) reflect.Value {
	if v.CanConvert(typ) {
		return v.Convert(typ)
	}
	return emptyValue
}

func singleValue[T any](v any) T {
	switch v := v.(type) {
	case []any:
		if len(v) > 0 {
			return singleValue[T](v[0])
		}
	case T:
		return v
	}
	var t T
	return t
}
