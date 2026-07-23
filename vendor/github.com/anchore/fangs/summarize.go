package fangs

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/anchore/go-logger"
)

var trailingSpace = regexp.MustCompile(`[ \r]+\n`)

func Summarize(cfg Config, descriptions DescriptionProvider, filter ValueFilterFunc, values ...any) string {
	root := &section{}
	for _, value := range values {
		v := reflect.ValueOf(value)
		summarize(cfg, descriptions, root, v, nil)
	}
	if filter == nil {
		filter = func(s string) string {
			return s
		}
	}

	return root.stringify(cfg, filter)
}

func SummarizeCommand(cfg Config, cmd *cobra.Command, filter ValueFilterFunc, values ...any) string {
	root := cmd
	for root.Parent() != nil {
		root = root.Parent()
	}
	descriptions := DescriptionProviders(
		NewFieldDescriber(values...),
		NewStructDescriptionTagProvider(),
		NewCommandFlagDescriptionProvider(cfg.TagName, root),
	)
	return Summarize(cfg, descriptions, filter, values...)
}

func SummarizeLocations(cfg Config) (out []string) {
	for _, f := range cfg.Finders {
		out = append(out, f(cfg)...)
	}
	return
}

type ValueFilterFunc func(string) string

func summarize(cfg Config, descriptions DescriptionProvider, s *section, value reflect.Value, path []string) {
	v, t := base(value)

	if !isStruct(t) {
		panic(fmt.Sprintf("Summarize requires struct types, got: %#v", value.Interface()))
	}

	summarizeFields(cfg, descriptions, s, v, t, path)
}

func summarizeFields(cfg Config, descriptions DescriptionProvider, s *section, v reflect.Value, t reflect.Type, path []string) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !includeField(f) {
			continue
		}

		currentPath := path
		name := f.Name

		// should we ignore this field based on the output tag?
		if cfg.TagName != "yaml" {
			if tag, ok := f.Tag.Lookup("yaml"); ok {
				parts := strings.Split(tag, ",")
				if parts[0] == "-" {
					continue
				}
			}
		}

		if tag, ok := f.Tag.Lookup(cfg.TagName); ok {
			parts := strings.Split(tag, ",")
			tag = parts[0]
			if tag == "-" {
				continue
			}
			switch {
			case contains(parts, "squash"):
				name = ""
			case tag == "":
				currentPath = append(currentPath, name)
			default:
				name = tag
				currentPath = append(currentPath, tag)
			}
		} else {
			currentPath = append(currentPath, name)
		}

		// process the field based on its type
		summarizeField(cfg, descriptions, s, f, v.Field(i), name, currentPath)
	}
}

// summarizeField handles a single field according to its type
func summarizeField(cfg Config, descriptions DescriptionProvider, s *section, f reflect.StructField, fieldValue reflect.Value, fieldName string, path []string) {
	v, t := base(fieldValue)

	if isStruct(t) {
		sub := s
		if fieldName != "" {
			sub = s.sub(fieldName)
		}

		if isPtr(v.Type()) && v.IsNil() {
			v = reflect.New(t)
		}

		summarize(cfg, descriptions, sub, v, path)
		return
	}

	// handle non-struct fields...

	env := envVar(cfg.AppName, path...)

	// for slices of structs, do not output an env var
	if t.Kind() == reflect.Slice && baseType(t.Elem()).Kind() == reflect.Struct {
		env = ""
	}

	s.add(cfg.Logger,
		fieldName,
		fieldValue,
		descriptions.GetDescription(fieldValue, f),
		env)
}

// printVal prints a value in YAML format
func printVal(cfg Config, filter ValueFilterFunc, value reflect.Value, indent string) string {
	buf := bytes.Buffer{}

	v, t := base(value)
	switch {
	case isSlice(t):
		if v.Len() == 0 {
			return "[]"
		}

		for i := 0; i < v.Len(); i++ {
			v := v.Index(i)
			buf.WriteString("\n")
			buf.WriteString(indent)
			buf.WriteString("- ")

			val := printVal(cfg, filter, v, indent+"  ")
			val = strings.TrimSpace(val)
			buf.WriteString(val)

			// separate struct entries by an empty line
			_, t := base(v)
			if isStruct(t) {
				buf.WriteString("\n")
			}
		}

	case isStruct(t):
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !includeField(f) {
				continue
			}

			name := f.Name

			if tag, ok := f.Tag.Lookup(cfg.TagName); ok {
				parts := strings.Split(tag, ",")
				tag = parts[0]
				if tag == "-" {
					continue
				}
				switch {
				case contains(parts, "squash"):
					name = ""
				case tag == "":
				default:
					name = tag
				}
			}

			v := v.Field(i)

			buf.WriteString("\n")
			buf.WriteString(indent)

			val := printVal(cfg, filter, v, indent+"  ")

			val = fmt.Sprintf("%s: %s", name, val)

			buf.WriteString(val)
		}

	case v.CanInterface():
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return ""
		}
		if v.Kind() == reflect.String {
			return fmt.Sprintf("'%s'", filter(v.String()))
		}
		return filter(fmt.Sprintf("%v", v.Interface()))
	}

	val := buf.String()
	// for slices, there will be an extra newline, which we want to remove
	val = strings.TrimSuffix(val, "\n")
	return val
}

func base(v reflect.Value) (reflect.Value, reflect.Type) {
	t := v.Type()
	for isPtr(t) {
		t = t.Elem()
		if v.IsNil() {
			newV := reflect.New(t)
			// If the field we're looking at is nil, and is a pointer to a struct,
			// change it to point to an empty instance of the struct, so that we can
			// continue recursing on the config structure. However, if it's a nil pointer
			// to a primitive type, leave it as nil so that we can tell later in the summary
			// that it wasn't set.
			if newV.Kind() == reflect.Struct {
				v = newV
			}
		} else {
			v = v.Elem()
		}
	}
	return v, t
}

func baseType(t reflect.Type) reflect.Type {
	for isPtr(t) {
		t = t.Elem()
	}
	return t
}

type section struct {
	name        string
	value       reflect.Value
	description string
	env         string
	subsections []*section
}

func (s *section) get(name string) *section {
	for _, s := range s.subsections {
		if s.name == name {
			return s
		}
	}
	return nil
}

func (s *section) sub(name string) *section {
	sub := s.get(name)
	if sub == nil {
		sub = &section{
			name: name,
		}
		s.subsections = append(s.subsections, sub)
	}
	return sub
}

func (s *section) add(log logger.Logger, name string, value reflect.Value, description string, env string) *section {
	add := &section{
		name:        name,
		value:       value,
		description: description,
		env:         env,
	}
	sub := s.get(name)
	if sub != nil {
		if sub.name != name || !sub.value.CanConvert(value.Type()) || sub.description != description || sub.env != env {
			log.Warnf("multiple entries with different values: %#v != %#v", sub, add)
		}
		return sub
	}
	s.subsections = append(s.subsections, add)
	return add
}

func (s *section) stringify(cfg Config, filter ValueFilterFunc) string {
	out := &bytes.Buffer{}
	stringifySection(cfg, filter, out, s, "")

	// remove any extra trailing whitespace from final config
	return trailingSpace.ReplaceAllString(out.String(), "\n")
}

func stringifySection(cfg Config, filter ValueFilterFunc, out *bytes.Buffer, s *section, indent string) {
	nextIndent := indent

	if s.name != "" {
		nextIndent += "  "

		if s.description != "" {
			// support multi-line descriptions
			lines := strings.Split(strings.TrimSpace(s.description), "\n")
			for idx, line := range lines {
				out.WriteString(indent + "# " + line)
				if idx < len(lines)-1 {
					out.WriteString("\n")
				}
			}
		}
		if s.env != "" {
			value := fmt.Sprintf("(env: %s)", s.env)
			if s.description == "" {
				// since there is no description, we need to start the comment
				out.WriteString(indent + "# ")
			} else {
				// buffer between description and env hint
				out.WriteString(" ")
			}
			out.WriteString(value)
		}
		if s.description != "" || s.env != "" {
			out.WriteString("\n")
		}

		out.WriteString(indent)

		out.WriteString(s.name)
		out.WriteString(":")

		if s.value.IsValid() {
			val := printVal(cfg, filter, s.value, indent+"  ")
			if val != "" {
				out.WriteString(" ")
			}
			out.WriteString(val)
		}

		out.WriteString("\n")
	}

	for _, s := range s.subsections {
		stringifySection(cfg, filter, out, s, nextIndent)
		if len(s.subsections) == 0 {
			out.WriteString(nextIndent)
			out.WriteString("\n")
		}
	}
}
