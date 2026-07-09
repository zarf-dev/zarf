package expr

// Package represents a package value.
type Package struct {
	types   map[string]Type
	symbols map[string]Value
}

// NewPackage creates a new package.
func NewPackage(symbols map[string]Value) Package {
	pkg := Package{symbols: symbols, types: map[string]Type{}}
	for key := range symbols {
		pkg.types[key] = pkg.symbols[key].Type()
	}
	return pkg
}

// Symbol returns a symbol, or nil if the symbol doesn't exist.
func (p Package) Symbol(ident string) Value {
	if symbol, ok := p.symbols[ident]; ok {
		return symbol
	}
	return nil
}

// Type returns the type for this package.
func (p Package) Type() Type {
	return NewPackageType(p.types)
}
