package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/spdx/tools-golang/spdx/v3/internal/ld"
)

var Debug = ld.Debug

type ldContextProvider interface {
	LDContexts() map[string]map[string]any
}

func ToJSON(spdxContext string, ldContext ld.Context, document any, idGenerator IdGeneratorFunc, writer io.Writer) error {
	// we only compact the spdx context, not extensions, so ignore any other contexts
	ctx := ldContext.(ldContextProvider).LDContexts()[spdxContext]
	if ctx == nil {
		return fmt.Errorf("spdx context %v not found", spdxContext)
	}

	ctx, _ = ctx[ld.JsonContextProp].(map[string]any)
	if ctx == nil {
		return fmt.Errorf("spdx @context %v not found", spdxContext)
	}

	aliases := map[string]string{}
	prefixes := map[string]string{}

	for k, v := range ctx {
		switch v := v.(type) {
		case string:
			aliases[v] = k
		case map[string]any:
			fieldIRI, _ := v[ld.JsonIdProp].(string)
			if fieldIRI != "" {
				aliases[fieldIRI] = k
				subContext, _ := v[ld.JsonContextProp].(map[string]any)
				if subContext != nil {
					prefix, _ := subContext[ld.JsonVocabProp].(string)
					if prefix != "" {
						prefixes[fieldIRI] = prefix
					}
				}
			}
		}
	}

	s := &serializer{
		contextURL:  spdxContext,
		aliases:     aliases,
		prefixes:    prefixes,
		idGenerator: idGenerator,
		ids:         map[reflect.Value]string{},
		in:          document,
	}

	maps, err := s.toMaps(document)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "")
	return enc.Encode(maps)
}

type IdGeneratorFunc func(id string, value reflect.Value) string

type serializer struct {
	contextURL  string
	idGenerator IdGeneratorFunc
	ids         map[reflect.Value]string
	aliases     map[string]string
	prefixes    map[string]string
	graph       []any
	in          any
}

func (s *serializer) toMaps(document any) (map[string]any, error) {
	_, errs := s.serialize(reflect.ValueOf(document), "")
	return map[string]any{
		"@context": s.contextURL,
		"@graph":   s.graph,
	}, errors.Join(errs...)
}

func (s *serializer) serialize(v reflect.Value, prefix string) (any, []error) {
	out, err := s.serializeValue(v, prefix)
	if sv, ok := out.(string); ok {
		return strings.TrimPrefix(sv, prefix), err
	}
	return out, err
}

func (s *serializer) serializeValue(v reflect.Value, prefix string) (any, []error) {
	id := s.ids[v]
	if id != "" {
		return id, nil
	}
	if !v.IsValid() {
		return nil, nil
	}
	ptrV := v
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, nil
		}
		id := s.ids[v]
		if id != "" {
			return id, nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String, reflect.Bool, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return s.serializeSingleValue(v)
	case reflect.Interface:
		return s.serialize(v.Elem(), prefix)
	case reflect.Slice:
		return s.serializeSlice(v, prefix)
	case reflect.Struct:
		return s.serializeStruct(ptrV, v)
	default:
		panic(fmt.Errorf("unsupported type: %v", v))
	}
}

func (s *serializer) serializeSingleValue(v reflect.Value) (any, []error) {
	return getValue(v), nil
}

func (s *serializer) serializeSlice(v reflect.Value, prefix string) (any, []error) {
	// if this function is called, we have a non-nil slice that may be empty, we always
	// want to return a slice rather than nil
	sz := v.Len()
	serialized := make([]any, 0, sz)
	var errs []error
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if Debug {
			tmp := fmt.Sprintf("%#v", elem)
			fmt.Println(tmp)
		}
		got, err := s.serialize(elem, prefix)
		if err != nil {
			errs = append(errs, err...)
		} else {
			serialized = append(serialized, got)
		}
	}
	return serialized, errs
}

func (s *serializer) serializeStruct(ptrV reflect.Value, v reflect.Value) (any, []error) {
	var errs []error
	data, ok := s.serializeDataStruct(v)
	if ok {
		return data, nil
	}

	typeName := s.typeName(ptrV)
	if typeName == "" {
		if Debug {
			tmp := fmt.Sprintf("no type for: %#v", ptrV)
			fmt.Println(tmp)
		}
		// structs without types must have an external ID
		id, err := ld.GetID(v)
		if err != nil {
			return "", []error{err}
		}
		if isExternalID(id) {
			return id, nil
		}
		panic(fmt.Errorf("unable to get typeName or ID for: %v", ptrV.Interface()))
	}

	serialized := map[string]any{}
	idField := ""
	errs = append(errs, s.serializeProps(ptrV, v, &idField, serialized)...)

	id, _ := serialized[idField].(string)
	switch len(serialized) {
	case 0:
		return nil, nil
	case 1:
		// if the only field output is an external ID, output directly as a string
		if isExternalID(id) {
			return id, nil
		}
	}

	typeField := s.aliases[ld.JsonTypeProp]
	if typeField == "" {
		typeField = ld.JsonTypeProp
	}

	serialized[typeField] = typeName

	// move elements to top-level graph
	if id != "" {
		// this struct needs to be serialized to the graph, and return the id reference to it
		s.graph = append(s.graph, serialized)
		return id, errs
	}

	return serialized, errs
}

func (s *serializer) serializeProps(ptrV reflect.Value, v reflect.Value, idField *string, serialized map[string]any) (errs []error) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)

		fv := v.Field(i)

		iri := f.Tag.Get(ld.GoIriTagName)
		if iri == "" {
			if f.Anonymous && f.Type.Kind() == reflect.Struct { // embedded parent type
				errs = append(errs, s.serializeProps(ptrV, fv, idField, serialized)...)
			}
			continue
		}

		k := s.aliases[iri]
		if k == "" {
			k = iri
		}

		if iri == ld.JsonIdProp {
			// some types must use @id and allow blank node IDs
			if blankNodeAllowed(ptrV.Type()) {
				//if ld.RefCount(ptrV, s.in) == 1 {
				//	continue // don't create an ID, output inline
				//}
				if outputInline(ptrV) {
					continue // don't create an ID, output inline
				}
				k = iri
			}
			*idField = k
			if isUnset(fv) {
				newID := s.idGenerator("", ptrV)
				s.ids[ptrV] = newID
				fv = reflect.ValueOf(newID)
			} else {
				id := fv.String()
				// SPDXRef-<value> many users will be outputting today, which are not valid URIs
				if !IsURI(id) {
					id = s.idGenerator(id, ptrV)
					fv.SetString(id)
				}
				s.ids[ptrV] = id
			}
		}

		if isUnset(fv) {
			if f.Tag.Get("required") != "true" {
				continue
			} else if f.Type.Kind() == reflect.Slice {
				// always output arrays instead of nil when required
				fv = reflect.MakeSlice(f.Type, 0, 0)
			}
		}

		serializedField, err := s.serialize(fv, s.prefixes[iri])
		if err != nil {
			errs = append(errs, err...)
			continue
		}

		serialized[k] = serializedField
	}
	return errs
}

func IsURI(id string) bool {
	if id == "" {
		return false
	}
	u, err := url.ParseRequestURI(id)
	return err == nil && u != nil
}

func outputInline(v reflect.Value) bool {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return true
	}
	t := v.Type()
	if t.Name() == "CreationInfo" || !blankNodeAllowed(t) {
		return false
	}
	return true
}

func blankNodeAllowed(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface:
		return blankNodeAllowed(t.Elem())
	case reflect.Struct:
		f, ok := ld.FieldByType[ld.Type](t)
		if ok {
			return strings.Contains(f.Tag.Get(ld.GoNodeKindTagName), "BlankNode")
		}
	default:
	}
	return false
}

func (s *serializer) getId(v reflect.Value) (string, error) {
	id, _ := ld.GetID(v)
	if id != "" {
		return id, nil
	}
	id = s.ids[v]
	return id, nil
}

func (s *serializer) typeName(v reflect.Value) string {
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	f, ok := ld.FieldByType[ld.Type](v.Type())
	if ok {
		iri := f.Tag.Get(ld.GoIriTagName)
		alias := s.aliases[iri]
		if alias != "" {
			return alias
		}
		return iri
	}
	return ""
}

func (s *serializer) serializeDataStruct(v reflect.Value) (any, bool) {
	if v.CanInterface() {
		switch val := v.Interface().(type) {
		case time.Time:
			return val.UTC().Format(time.RFC3339), true
		}
	}
	return nil, false
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

func isExternalID(id string) bool {
	return strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://")
}
