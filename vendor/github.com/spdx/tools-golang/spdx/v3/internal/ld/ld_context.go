package ld

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/piprate/json-gold/ld"
)

// Type is a 0-size data holder property type for type-level ld information
type Type struct{}

// Context is the holder for all known LD contexts and required definitions
type Context interface {
	Register(contextURI string, contextDefinition map[string]any, typesAndInstances ...any) Context
	Merge(ctx Context) Context
	ToJSON(writer io.Writer, graph ...any) error
	FromJSON(reader io.Reader) ([]any, error)
}

const (
	JsonIdProp        = "@id"
	JsonTypeProp      = "@type"
	JsonValueProp     = "@value"
	JsonContextProp   = "@context"
	JsonGraphProp     = "@graph"
	JsonVocabProp     = "@vocab"
	GoTypeField       = "_"
	GoIdField         = "ID"
	GoIriTagName      = "iri"
	GoTypeTagName     = "type"
	GoNodeKindTagName = "node-kind"
	GoRequiredTagName = "required"
)

type context struct {
	contextMap map[string]*serializationContext
	// iriToType contains full IRIs and aliases to the appropriate typeContext
	iriToType map[string]*typeContext
	// typeToContext contains references from the go type(s) to appropriate typeContext
	typeToContext map[reflect.Type]*typeContext
	// iriToInstance are directly registered instances in code
	iriToInstance map[string]reflect.Value
	// typeToExternalIriFunc holds registered functions to construct external placeholders IRIs
	typeToExternalIriFunc map[reflect.Type]func(string) reflect.Value
}

func NewContext() Context {
	return &context{
		contextMap:            map[string]*serializationContext{},
		iriToType:             map[string]*typeContext{},
		typeToContext:         map[reflect.Type]*typeContext{},
		iriToInstance:         map[string]reflect.Value{},
		typeToExternalIriFunc: map[reflect.Type]func(string) reflect.Value{},
	}
}

// Merge returns a new context, with the values from both contexts merged together
func (c *context) Merge(ctx Context) Context {
	c2 := ctx.(*context)
	return &context{
		contextMap:    merge(c.contextMap, c2.contextMap),
		iriToType:     merge(c.iriToType, c2.iriToType),
		typeToContext: merge(c.typeToContext, c2.typeToContext),
		iriToInstance: merge(c.iriToInstance, c2.iriToInstance),
	}
}

// Register registers types and aliases to be used when serializing/deserializing documents
func (c *context) Register(contextURI string, ldContext map[string]any, types ...any) Context {
	ctx := c.getContext(contextURI)
	ctx.ldContext = merge(ctx.ldContext, ldContext)
	ctx.parsedLdContext = ld.NewContext(ldContext, nil)
	c.registerContextAliases(ctx, ldContext)
	for _, typ := range types {
		switch {
		case isFunc(typ):
			registerFunc(c, typ)
		default:
			registerType(c, ctx, typ)
		}
	}
	return c
}

// TypeAliases returns all the registered types and IRIs with corresponding aliases
func (c *context) TypeAliases() ([]reflect.Type, map[string]string) {
	iriToAlias := map[string]string{}
	for _, cm := range c.contextMap {
		for alias, iri := range cm.aliasToIri {
			iriToAlias[iri] = alias
		}
	}
	var out []reflect.Type
	for t := range c.typeToContext {
		out = append(out, t)
	}
	return out, iriToAlias
}

// LDContexts returns all the registered JSON-LD contexts
func (c *context) LDContexts() map[string]map[string]any {
	out := map[string]map[string]any{}
	for uri, cm := range c.contextMap {
		out[uri] = cm.ldContext
	}
	return out
}

// registerContextAliases registers compact name aliases for the given IRIs in the given context
func (c *context) registerContextAliases(ctx *serializationContext, ldContext map[string]any) {
	subContext, _ := ldContext[JsonContextProp].(map[string]any)
	if subContext != nil {
		c.registerContextAliases(ctx, subContext)
		return
	}
	for alias, v := range ldContext {
		if alias == JsonContextProp {
			continue
		}
		switch v := v.(type) {
		case string:
			c.registerContextAlias(ctx, v, alias)
		case map[string]any:
			iri, _ := v[JsonIdProp].(string)
			if iri != "" {
				c.registerContextAlias(ctx, iri, alias)
			}
			// should this be checked? if v[JsonTypeProp] == JsonVocabProp {
			subContext, _ = v[JsonContextProp].(map[string]any)
			if subContext != nil {
				contextPrefix, _ := subContext[JsonVocabProp].(string)
				ctx.aliasContext[alias] = contextPrefix
			}
		}
	}
}

// registerContextAlias registers compact name aliases for the given IRIs in the given context
func (c *context) registerContextAlias(ctx *serializationContext, iri, alias string) {
	if ctx.aliasToIri[alias] != "" {
		panic("duplicate alias set globally: " + alias + "; iri: " + iri + "; existing: " + ctx.aliasToIri[alias])
	}
	ctx.aliasToIri[alias] = iri
}

func (c *context) getContext(contextUrl string) *serializationContext {
	ctx := c.contextMap[contextUrl]
	if ctx == nil {
		ctx = &serializationContext{
			contextUrl:   contextUrl,
			aliasToIri:   map[string]string{},
			aliasContext: map[string]string{},
			//iriToAlias:   map[string]string{},
		}
		c.contextMap[contextUrl] = ctx
	}
	return ctx
}

func (c *context) ToJSON(writer io.Writer, graph ...any) error {
	out, err := c.toMaps(graph...)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (c *context) toMaps(graph ...any) (values map[string]any, errors error) {
	builder := graphBuilder{
		ctx:         c,
		nextID:      map[reflect.Type]int{},
		ids:         map[uintptr]string{},
		pointerRefs: map[reflect.Value]map[string]any{},
	}
	return builder.toCompactMaps(graph...)
}

func (c *context) FromJSON(reader io.Reader) ([]any, error) {
	var decoded any
	dec := json.NewDecoder(reader)
	err := dec.Decode(&decoded)
	if err != nil {
		return nil, err
	}
	switch values := decoded.(type) {
	case map[string]any:
		return c.fromMaps(values)
	case []any:
		return c.fromSlice(values)
	}
	return nil, fmt.Errorf("unable to decode, unsupported JSON type: %v", decoded)
}

func (c *context) fromMaps(values map[string]any) ([]any, error) {
	rdr := mapReader{ctx: c}
	return rdr.FromMaps(values)
}

func (c *context) fromSlice(values []any) ([]any, error) {
	rdr := mapReader{ctx: c}
	return rdr.FromSlice(values)
}

type typeContext struct {
	ctx              *serializationContext
	typ              reflect.Type
	iri              string
	alias            string
	setters          map[string]func(instance reflect.Value, value reflect.Value)
	blankNodeAllowed bool
}

type serializationContext struct {
	contextUrl string
	// the full JSON LD context provided
	ldContext       map[string]any
	parsedLdContext *ld.Context
	// aliasToIri contains field aliases to the respective IRIs
	aliasToIri map[string]string
	// aliasContext
	aliasContext map[string]string
	//iriToAlias   map[string]string
}

func registerFunc(c *context, fn any) {
	f := reflect.ValueOf(fn)
	t := f.Type()

	if t.NumIn() != 1 || t.In(0).Kind() != reflect.String {
		panic("external IRI functions must have one parameter, accepting an IRI string")
	}

	if t.NumOut() != 1 {
		panic("external IRI functions must have one return value")
	}

	rVal := t.Out(0)
	c.typeToExternalIriFunc[rVal] = func(s string) reflect.Value {
		out := f.Call([]reflect.Value{reflect.ValueOf(s)})
		return out[0]
	}
}

func registerType(c *context, ctx *serializationContext, instancePointer any) {
	t := reflect.TypeOf(instancePointer)
	instance := reflect.ValueOf(instancePointer)
	t = baseType(t) // types may be passed as pointers, but we want the base types

	tc := c.typeToContext[t]
	if tc == nil {
		meta, ok := FieldByType[Type](t)
		if ok {
			iri := meta.Tag.Get(GoIriTagName)
			if iri == "" {
				panic("no type IRI specified for: " + stringify(instancePointer))
			}
			tc = &typeContext{
				iri:              iri,
				ctx:              ctx,
				typ:              t,
				setters:          map[string]func(instance reflect.Value, value reflect.Value){},
				blankNodeAllowed: strings.Contains(meta.Tag.Get(GoNodeKindTagName), "BlankNode"),
			}
			c.iriToType[tc.iri] = tc
			c.typeToContext[t] = tc
		}
	}

	// capture all the registered types
	id, err := getID(instance)
	if err != nil {
		// we should not have invalid types registered
		panic(err)
	}
	if id != "" {
		switch instance.Type().Kind() {
		case reflect.Pointer, reflect.Struct:
		default:
			panic("expected instance registration to be a pointer or a struct, got: " + stringify(instance))
		}
		c.iriToInstance[id] = instance
	}
}
