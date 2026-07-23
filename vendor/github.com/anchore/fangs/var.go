package fangs

import (
	"strconv"
	"strings"

	"github.com/spf13/pflag"
)

// boolPtr is a pointer to a bool pointer field within a struct
type boolPtr struct {
	value **bool // consistent name with other pflag.Value types so FieldByName finds it
}

func (b *boolPtr) String() string {
	if b.value == nil {
		return ""
	}
	if *b.value == nil {
		return ""
	}
	if **b.value {
		return "true"
	}
	return "false"
}

func (b *boolPtr) Set(s string) error {
	if s == "" {
		*b.value = nil
		return nil
	}

	b1, err := strconv.ParseBool(s)
	if err == nil {
		*b.value = &b1
	}
	return nil
}

func (b *boolPtr) Type() string {
	return "bool"
}

var _ pflag.Value = (*boolPtr)(nil)

// BoolPtrVarP adds a boolean pointer flag with no default
func BoolPtrVarP(flags *pflag.FlagSet, ptr **bool, name string, short string, usage string) {
	flag := flags.VarPF(&boolPtr{
		value: ptr,
	}, name, short, usage)
	if *ptr == nil || !**ptr {
		flag.NoOptDefVal = "true"
	} else {
		flag.NoOptDefVal = "false"
	}
}

// stringPtr is a pointer to a string pointer field within a struct
type stringPtr struct {
	value **string // consistent name with other pflag.Value types so FieldByName finds it
}

func (b *stringPtr) String() string {
	if b.value == nil {
		return ""
	}
	if *b.value == nil {
		return ""
	}
	return **b.value
}

func (b *stringPtr) Set(s string) error {
	if s == "" {
		*b.value = nil
		return nil
	}
	*b.value = &s
	return nil
}

func (b *stringPtr) Type() string {
	return "*string"
}

var _ pflag.Value = (*stringPtr)(nil)

// StringPtrVarP adds a string pointer flag with no default
func StringPtrVarP(flags *pflag.FlagSet, ptr **string, name string, short string, usage string) {
	flags.VarP(&stringPtr{
		value: ptr,
	}, name, short, usage)
}

// intPtr is a pointer to an int pointer field within a struct
type intPtr struct {
	value **int // consistent name with other pflag.Value types so FieldByName finds it
}

func (b *intPtr) String() string {
	if b.value == nil {
		return ""
	}
	if *b.value == nil {
		return ""
	}
	return strconv.Itoa(**b.value)
}

func (b *intPtr) Set(s string) error {
	if s == "" {
		*b.value = nil
		return nil
	}
	v, err := strconv.Atoi(s)
	if err == nil {
		*b.value = &v
	}
	return nil
}

func (b *intPtr) Type() string {
	return "*int"
}

var _ pflag.Value = (*intPtr)(nil)

// IntPtrVarP adds an int pointer flag with no default
func IntPtrVarP(flags *pflag.FlagSet, ptr **int, name string, short string, usage string) {
	flags.VarP(&intPtr{
		value: ptr,
	}, name, short, usage)
}

// intPtr is a pointer to a float64 pointer field within a struct
type float64Ptr struct {
	value **float64 // consistent name with other pflag.Value types so FieldByName finds it
}

func (b *float64Ptr) String() string {
	if b.value == nil {
		return ""
	}
	if *b.value == nil {
		return ""
	}

	return strconv.FormatFloat(**b.value, 'f', -1, 64)
}

func (b *float64Ptr) Set(s string) error {
	if s == "" {
		*b.value = nil
		return nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err == nil {
		*b.value = &v
	}
	return nil
}

func (b *float64Ptr) Type() string {
	return "*float64"
}

var _ pflag.Value = (*float64Ptr)(nil)

// Float64PtrVarP adds a float64 pointer flag with no default
func Float64PtrVarP(flags *pflag.FlagSet, ptr **float64, name string, short string, usage string) {
	flags.VarP(&float64Ptr{
		value: ptr,
	}, name, short, usage)
}
