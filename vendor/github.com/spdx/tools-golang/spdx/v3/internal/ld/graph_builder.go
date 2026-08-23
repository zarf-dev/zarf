package ld

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/piprate/json-gold/ld"
)

type graphBuilder struct {
	ctx         *context
	input       reflect.Value
	graph       []any // graph stores all the serialized objects in the graph
	nextID      map[reflect.Type]int
	ids         map[uintptr]string
	pointerRefs map[reflect.Value]map[string]any // pointerRefs stores references to each serialized pointer
}

func (b *graphBuilder) toCompactMaps(graph ...any) (map[string]any, error) {
	expanded, errs := b.toExpandedMaps(graph...)
	if errs != nil {
		return nil, errors.Join(errs...)
	}

	proc := ld.NewJsonLdProcessor()
	opts := ld.NewJsonLdOptions("")
	// all options:
	//opts.Base
	opts.CompactArrays = true
	//opts.ExpandContext = false
	opts.ProcessingMode = ld.JsonLd_1_1
	opts.DocumentLoader = offlineDocumentLoader{ctx: b.ctx}
	//opts.Embed
	//opts.Explicit
	//opts.RequireAll
	//opts.FrameDefault = false
	//opts.OmitDefault
	//opts.OmitGraph
	//opts.UseRdfType
	//opts.UseNativeTypes = true
	//opts.ProduceGeneralizedRdf
	//opts.InputFormat
	//opts.Format
	//opts.Algorithm
	//opts.UseNamespaces
	//opts.OutputForm
	//opts.SafeMode

	var compactionContext map[string]any
	switch len(b.ctx.contextMap) {
	case 0:
		return nil, fmt.Errorf("no contexts defined, unable to serialize")
	case 1:
		compactionContext = map[string]interface{}{
			"@context": firstKey(b.ctx.contextMap),
		}
	default:
		prefixes := map[string]any{}
		for i, url := range sortedKeys(b.ctx.contextMap) {
			prefixes["ns"+strconv.Itoa(i)] = url
		}
		compactionContext = map[string]interface{}{
			"@context": prefixes,
		}
	}

	compact, err := proc.Compact(expanded, compactionContext, opts)
	return compact, err
}

func (b *graphBuilder) toExpandedMaps(graph ...any) ([]any, []error) {
	b.input = reflect.ValueOf(graph)
	b.graph = nil
	for _, v := range graph {
		val := reflect.ValueOf(v)
		_, err := b.serialize(val)
		if err != nil {
			return nil, err
		}
	}
	return b.graph, nil
}

// serialize outputs the top-level nodes in the graph; these have the behavior that they are always returned in
// serialized form rather than potentially returning an ID reference. pointers with multiple references will also
// ensure the @id field is set in order to be referenced later
func (b *graphBuilder) serialize(v reflect.Value) (any, []error) {
	if !v.IsValid() {
		return nil, nil
	}
	ptrV := v
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, nil
		}
		if Debug {
			ptr := fmt.Sprintf("%v", v.Pointer())
			fmt.Printf("got pointer: %v\n", ptr)
		}
		id := b.ids[v.Pointer()]
		if id != "" {
			return id, nil
		}
		v = v.Elem()
	}

	val, ok := b.serializePrimitiveValue(v)
	if ok {
		return val, nil
	}

	switch v.Kind() {
	case reflect.Interface:
		return b.serialize(v.Elem())
	case reflect.Slice:
		return b.serializeSlice(v)
	case reflect.Struct:
		return b.serializeStruct(ptrV, v)
	default:
		panic(fmt.Errorf("unsupported type: %v", v))
	}
}

func (b *graphBuilder) serializeSlice(slice reflect.Value) ([]any, []error) {
	if slice.Kind() != reflect.Slice {
		panic("expected slice")
	}
	var out []any
	for i := 0; i < slice.Len(); i++ {
		value, err := b.serialize(slice.Index(i))
		if err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, nil
}

func (b *graphBuilder) serializeStruct(ptrV, v reflect.Value) (value any, err []error) {
	t := v.Type()
	if t.Kind() != reflect.Struct {
		panic("expected struct, got: " + stringify(v))
	}

	out := map[string]any{}

	tc := b.ctx.typeToContext[t]
	if tc != nil {
		err = b.serializeProps(tc.ctx, tc, ptrV, v, out)
		if err != nil {
			return out, err
		}

		// always append the type unless the only value we have is an external IRI reference
		if len(out) > 0 {
			out[JsonTypeProp] = []any{
				tc.iri,
			}
		}
	}

	id := out[JsonIdProp]
	if id != "" {
		b.graph = append(b.graph, out)
		return id, nil
	}

	// skip objects with no properties whatsoever
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (b *graphBuilder) serializeProps(context *serializationContext, tc *typeContext, ptrV, v reflect.Value, out map[string]any) []error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if skipField(f) { // ID is set outside of this function
			continue
		}

		prop := f.Tag.Get(GoIriTagName)
		fieldV := v.Field(i)

		if prop == JsonIdProp {
			if tc.blankNodeAllowed {
				if RefCount(ptrV, b.input) == 1 {
					continue // don't create an ID, output inline
				}
			}
			id := ""
			if isUnset(fieldV) {
				id = b.getID(ptrV)
				if Debug {
					val := ptrV.Interface()
					fmt.Printf("%#v\n", val)
				}
			} else {
				id = fieldV.String()
			}
			if Debug {
				ptr := fmt.Sprintf("%v", ptrV.Pointer())
				fmt.Printf("setting id for pointer: %v\n", ptr)
			}
			if ptrV.Kind() == reflect.Pointer {
				b.ids[ptrV.Pointer()] = id
			}
			out[JsonIdProp] = id
			continue
		}

		// embedded struct, recursively call this function to get all struct values
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			err := b.serializeProps(context, tc, ptrV, v.Field(i), out)
			if err != nil {
				return err
			}
			continue
		}

		optional := !isRequired(f)

		if optional && isEmpty(fieldV) {
			continue
		}

		if Debug {
			val := fieldV.Interface()
			str := fmt.Sprintf("serializing prop %v: %#v", f, val)
			fmt.Println(str)
		}
		val, err := b.serialize(fieldV)
		if err != nil {
			return err
		}

		if val == nil && optional {
			continue
		}

		out[prop] = val
	}

	return nil
}

func (b *graphBuilder) serializePrimitiveValue(v reflect.Value) (map[string]any, bool) {
	if !v.IsValid() || !v.CanInterface() {
		return nil, false
	}

	value := v.Interface()
	c := typeToConverter[v.Type()]
	if c != nil {
		if c.Serialize != nil {
			value = c.Serialize(value)
		}
		return map[string]any{
			JsonTypeProp:  c.IRI,
			JsonValueProp: value,
		}, true
	}

	return nil, false
}

// getID will return an ID for the given struct pointer, creating one if needed
// it does not append structs to the graph
func (b *graphBuilder) getID(ptrV reflect.Value) string {
	if ptrV.Type().Kind() != reflect.Pointer {
		panic("expected pointer, got: " + stringify(ptrV))
	}
	id, _ := b.ids[ptrV.Pointer()]
	if id != "" {
		return id
	}

	v := ptrV.Elem()
	t := v.Type()

	// check if the struct has an ID set directly, and use that if so
	id, _ = getID(v)
	if id != "" {
		return id
	}

	nextID := b.nextID[t] + 1
	b.nextID[t] = nextID
	return fmt.Sprintf("_:%s-%v", t.Name(), nextID)
}

func (b *graphBuilder) findContext(t reflect.Type) *serializationContext {
	t = baseType(t) // map[string]any may be a pointer, but we want the base types
	tc := b.ctx.typeToContext[t]
	if tc != nil {
		return tc.ctx
	}
	return nil
}

func stringify(o any) string {
	switch o := o.(type) {
	case reflect.Value:
		if !o.IsValid() {
			return "<invalid reflect value>"
		}
		if o.CanInterface() {
			return typeName(o.Type()) + ": " + stringify(o.Interface())
		}
	case reflect.Type:
		return fmt.Sprintf("%s.%s", o.PkgPath(), o.Name())
	}
	return fmt.Sprintf("%#v", o)
}

func isEmpty(v reflect.Value) bool {
	return !v.IsValid() || v.IsZero()
}

func isRequired(f reflect.StructField) bool {
	return f.Tag.Get("required") == "true"
}

func getValue(v reflect.Value) any {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return v.Bool()
	case reflect.Float32, reflect.Float64:
		return v.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	default:
		if !v.CanInterface() {
			return nil
		}
		return v.Interface()
	}
}

func isUnset(fv reflect.Value) bool {
	if !fv.IsValid() {
		return true
	}
	switch fv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return fv.IsNil()
	default:
		return fv.IsZero()
	}
}
