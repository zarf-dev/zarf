// Package jsonpath is an implementation of http://goessner.net/articles/JsonPath/
// If a JSONPath contains one of
// [key1, key2 ...], .., *, [min:max], [min:max:step], (? expression)
// all matchs are listed in an []interface{}
//
// The package comes with an extension of JSONPath to access the wildcard values of a match.
// If the JSONPath is used inside of a JSON object, you can use placeholder '#' or '#i' with natural number i
// to access all wildcards values or the ith wildcard
//
// This package can be extended with gval modules for script features like multiply, length, regex or many more.
// So take a look at github.com/Intevation/gval.
package jsonpath

import (
	"context"

	"github.com/Intevation/gval"
)

type Array interface {
	gval.Selector

	Len() int
	ForEach(func(key string, v interface{}))
}

type Object interface {
	gval.Selector

	ForEach(func(key string, v interface{}))
}

// New returns an selector for given JSONPath
func New(path string) (gval.Evaluable, error) {
	return lang.NewEvaluable(path)
}

// Get executes given JSONPath on given value
func Get(path string, value interface{}) (interface{}, error) {
	eval, err := lang.NewEvaluable(path)
	if err != nil {
		return nil, err
	}
	return eval(context.Background(), value)
}

var lang = func() gval.Language {
	l := gval.NewLanguage(
		gval.Base(),
		gval.PrefixExtension('$', parseRootPath),
		gval.PrefixExtension('@', parseCurrentPath),
	)
	l.CreateScanner(CreateScanner)
	return l
}()

// Language is the JSONPath Language
func Language() gval.Language {
	return lang
}

var placeholderExtension = func() gval.Language {
	l := gval.NewLanguage(
		lang,
		gval.PrefixExtension('{', parseJSONObject),
		gval.PrefixExtension('#', parsePlaceholder),
	)
	l.CreateScanner(CreateScanner)
	return l
}()

// PlaceholderExtension is the JSONPath Language with placeholder
func PlaceholderExtension() gval.Language {
	return placeholderExtension
}
