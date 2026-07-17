package fangs

import (
	"github.com/spf13/pflag"

	"github.com/anchore/go-logger"
)

// FlagSet is a facade of pflag.FlagSet, as fangs requires all flag add calls to use field references
// in order to match reading configuration and summarization information.
// The methods do not take default values, however, which should be set on the struct directly.
// There is one additional method: BoolPtrVarP, which allows for adding flags for bool pointers, needed by some
// multi-level configurations.
type FlagSet interface {
	BoolVarP(p *bool, name, shorthand, usage string)
	BoolPtrVarP(p **bool, name, shorthand, usage string)
	Float64VarP(p *float64, name, shorthand, usage string)
	CountVarP(p *int, name, shorthand, usage string)
	IntVarP(p *int, name, shorthand, usage string)
	StringVarP(p *string, name, shorthand, usage string)
	StringArrayVarP(p *[]string, name, shorthand, usage string)
}

// PFlagSetProvider provides access to the underlying pflag.FlagSet; the FlagSet may be type asserted to this interface
//
// WARNING: only use this when the fangs API does not provide a necessary feature, such as marking a flag as deprecated.
// Using the pflag.FlagSet directly may result in a mismatch between flags and configuration.
type PFlagSetProvider interface {
	PFlagSet() *pflag.FlagSet
}

type pflagSet struct {
	ignoreDuplicates bool
	log              logger.Logger
	flagSet          *pflag.FlagSet
}

var _ interface {
	FlagSet
	PFlagSetProvider
} = (*pflagSet)(nil)

func NewPFlagSet(log logger.Logger, flags *pflag.FlagSet) FlagSet {
	return &pflagSet{
		ignoreDuplicates: false,
		log:              log,
		flagSet:          flags,
	}
}

func (f *pflagSet) PFlagSet() *pflag.FlagSet {
	return f.flagSet
}

func (f *pflagSet) exists(name, shorthand string) bool {
	if !f.ignoreDuplicates {
		return false
	}
	if f.flagSet.Lookup(name) != nil {
		f.log.Debugf("flag already set: %s", name)
		return true
	}
	if shorthand != "" && f.flagSet.ShorthandLookup(shorthand) != nil {
		f.log.Debugf("flag shorthand already set: %s", shorthand)
		return true
	}
	return false
}

func (f *pflagSet) BoolVarP(p *bool, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	f.flagSet.BoolVarP(p, name, shorthand, *p, usage)
}

func (f *pflagSet) BoolPtrVarP(p **bool, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	BoolPtrVarP(f.flagSet, p, name, shorthand, usage)
}

func (f *pflagSet) Float64VarP(p *float64, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	f.flagSet.Float64VarP(p, name, shorthand, *p, usage)
}

func (f *pflagSet) CountVarP(p *int, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	f.flagSet.CountVarP(p, name, shorthand, usage)
}

func (f *pflagSet) IntVarP(p *int, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	f.flagSet.IntVarP(p, name, shorthand, *p, usage)
}

func (f *pflagSet) StringVarP(p *string, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	f.flagSet.StringVarP(p, name, shorthand, *p, usage)
}

func (f *pflagSet) StringArrayVarP(p *[]string, name, shorthand, usage string) {
	if f.exists(name, shorthand) {
		return
	}
	var val []string
	if p != nil {
		val = *p
	}
	f.flagSet.StringArrayVarP(p, name, shorthand, val, usage)
}
