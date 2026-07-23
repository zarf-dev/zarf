package expr

import "reflect"

// Assertions.
var (
	_ = TypeResolver(&TypeResolverAdapter{})
	_ = TypeResolver(&MetaTypeResolver{})
	_ = Resolver(&MetaResolver{})
	_ = TypeResolver(&StructTypeResolver{})
	_ = Resolver(&StructResolver{})
	_ = TypeResolver(&MapTypeResolver{})
	_ = Resolver(&MapResolver{})
)

// TypeResolver resolves types.
type TypeResolver interface {
	TypeResolve(ident string) Type
}

// Resolver resolves runtime values.
type Resolver interface {
	Resolve(ident string) Value
}

// TypeResolverAdapter adapts a runtime resolver to a type resolver by taking
// types of values retrieve from Resolve.
type TypeResolverAdapter struct {
	Resolver
}

// NewTypeResolverAdapter creates a new TypeResolverAdapter from a resolver.
func NewTypeResolverAdapter(r Resolver) *TypeResolverAdapter {
	return &TypeResolverAdapter{r}
}

// TypeResolve implements TypeResolver.
func (r *TypeResolverAdapter) TypeResolve(ident string) Type {
	return r.Resolve(ident).Type()
}

// MetaTypeResolver runs multiple type resolvers serially.
type MetaTypeResolver struct {
	resolvers []TypeResolver
}

// NewMetaTypeResolver creates a new meta type resolver.
func NewMetaTypeResolver() *MetaTypeResolver {
	return &MetaTypeResolver{}
}

// AddResolver adds a new resolver below other resolvers.
func (r *MetaTypeResolver) AddResolver(n TypeResolver) {
	r.resolvers = append(r.resolvers, n)
}

// TypeResolve implements TypeResolver.
func (r *MetaTypeResolver) TypeResolve(ident string) Type {
	for _, resolver := range r.resolvers {
		if t := resolver.TypeResolve(ident); t != nil {
			return t
		}
	}
	return nil
}

// MetaResolver runs multiple resolvers serially.
type MetaResolver struct {
	resolvers []Resolver
}

// NewMetaResolver creates a new meta resolver.
func NewMetaResolver() *MetaResolver {
	return &MetaResolver{}
}

// AddResolver adds a new resolver below other resolvers.
func (r *MetaResolver) AddResolver(n Resolver) {
	r.resolvers = append(r.resolvers, n)
}

// Resolve implements Resolver.
func (r *MetaResolver) Resolve(ident string) Value {
	for _, resolver := range r.resolvers {
		if t := resolver.Resolve(ident); t != nil {
			return t
		}
	}
	return nil
}

// StructTypeResolver resolves types of struct fields.
type StructTypeResolver struct {
	struc *StructType
}

// NewStructTypeResolver creates a new struct type resolver.
func NewStructTypeResolver(s interface{}) *StructTypeResolver {
	return &StructTypeResolver{TypeOf(s).(*StructType)}
}

// TypeResolve implements TypeResolver.
func (r *StructTypeResolver) TypeResolve(ident string) Type {
	if f, ok := r.struc.FieldByName(ident); ok {
		return f.Type
	}
	return nil
}

// StructResolver resolves struct fields.
type StructResolver struct {
	struc reflect.Value
}

// NewStructResolver creates a new struct resolver.
func NewStructResolver(s reflect.Value) *StructResolver {
	return &StructResolver{s}
}

// Resolve implements Resolver.
func (r *StructResolver) Resolve(ident string) Value {
	if sv := r.struc.FieldByName(ident); sv.IsValid() {
		return ValueOf(sv.Interface())
	}
	return nil
}

// MapTypeResolver resolves map keys.
type MapTypeResolver struct {
	m map[string]Type
}

// NewMapTypeResolver creates a new struct resolver.
func NewMapTypeResolver(m map[string]Type) *MapTypeResolver {
	return &MapTypeResolver{m}
}

// TypeResolve implements TypeResolver.
func (r *MapTypeResolver) TypeResolve(ident string) Type {
	if t, ok := r.m[ident]; ok {
		return t
	}
	return nil
}

// MapResolver resolves map keys.
type MapResolver struct {
	m map[string]Value
}

// NewMapResolver creates a new struct resolver.
func NewMapResolver(m map[string]Value) *MapResolver {
	return &MapResolver{m}
}

// Resolve implements Resolver.
func (r *MapResolver) Resolve(ident string) Value {
	if v, ok := r.m[ident]; ok {
		return v
	}
	return nil
}
