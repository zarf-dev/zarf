package sarif

// ToolComponentReference ...
type ToolComponentReference struct {
	PropertyBag
	Name  *string `json:"name"`
	Index *uint   `json:"index"`
	Guid  *string `json:"guid"`
}

// NewToolComponentReference ...
func NewToolComponentReference() *ToolComponentReference {
	return &ToolComponentReference{}
}

// WithName ...
func (t *ToolComponentReference) WithName(name string) *ToolComponentReference {
	t.Name = &name
	return t
}

// WithIndex ...
func (t *ToolComponentReference) WithIndex(index int) *ToolComponentReference {
	i := uint(index)
	t.Index = &i
	return t
}

// WithGuid ...
func (t *ToolComponentReference) WithGuid(guid string) *ToolComponentReference {
	t.Guid = &guid
	return t
}
