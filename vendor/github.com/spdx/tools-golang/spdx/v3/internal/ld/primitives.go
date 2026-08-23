package ld

import (
	"fmt"
	"net/url"
	"reflect"
	"time"
)

type URI string

func (u URI) Validate() error {
	if u == "" { // this is handled by required check
		return nil
	}
	_, err := url.Parse(string(u))
	return err
}

type PositiveInt int

func (i PositiveInt) Validate() error {
	if i < 0 {
		return fmt.Errorf("positive integer required, got: %v", i)
	}
	return nil
}

type NonNegativeInt int

func (i NonNegativeInt) Validate() error {
	if i < 0 {
		return fmt.Errorf("non-negative integer required, got: %v", i)
	}
	return nil
}

// DateTime is a specifically typed time.Time, validation inherently not needed
type DateTime time.Time

var converters = []converter{
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#string",
		Type: typeOf[string](),
	},
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#anyURI",
		Type: typeOf[URI](),
	},
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#integer",
		Type: typeOf[int](),
	},
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#positiveInteger",
		Type: typeOf[PositiveInt](),
	},
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#nonNegativeInteger",
		Type: typeOf[NonNegativeInt](),
	},
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#boolean",
		Type: typeOf[bool](),
	},
	{
		IRI:  "http://www.w3.org/2001/XMLSchema#decimal",
		Type: typeOf[float64](),
	},
	{
		IRI:         "http://www.w3.org/2001/XMLSchema#dateTime",
		Type:        typeOf[DateTime](),
		Serialize:   serializeTime[DateTime],
		Deserialize: deserializeTime[DateTime],
	},
	{
		IRI:         "http://www.w3.org/2001/XMLSchema#dateTimeStamp",
		Type:        typeOf[time.Time](),
		Serialize:   serializeTime[time.Time],
		Deserialize: deserializeTime[time.Time],
	},
}

var iriToConverter = getIriToConverter()

var typeToConverter = getTypeToConverter()

func TypeForIRI(iri string) reflect.Type {
	c := iriToConverter[iri]
	if c != nil {
		return c.Type
	}
	return nil
}

func typeOf[T any]() reflect.Type {
	var t T
	return reflect.TypeOf(t)
}

type converter struct {
	Type        reflect.Type
	IRI         string
	Serialize   func(any) any
	Deserialize func(any) any
}

func getTypeToConverter() map[reflect.Type]*converter {
	out := map[reflect.Type]*converter{}
	for i := range converters {
		c := &converters[i]
		if _, ok := out[c.Type]; ok {
			continue
		}
		out[c.Type] = c
	}
	return out
}

func getIriToConverter() map[string]*converter {
	out := map[string]*converter{}
	for i := range converters {
		c := &converters[i]
		if c.IRI == "" {
			panic("no IRI set")
		}
		if _, ok := out[c.IRI]; ok {
			panic("duplicate IRI set: " + c.IRI)
		}
		out[c.IRI] = c
	}
	return out
}

func serializeTime[T time.Time | DateTime](goValue any) any {
	if t, ok := goValue.(T); ok {
		return time.Time(t).Format(time.RFC3339)
	}
	return nil
}

func deserializeTime[T time.Time | DateTime](incoming any) any {
	s, _ := incoming.(string)
	if s != "" {
		parsed, _ := time.Parse(time.RFC3339, s)
		return T(parsed)
	}
	return nil
}
