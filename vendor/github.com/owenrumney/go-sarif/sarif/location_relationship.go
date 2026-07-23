package sarif

// LocationRelationship ...
type LocationRelationship struct {
	PropertyBag
	Target      uint     `json:"target"`
	Kinds       []string `json:"kinds,omitempty"`
	Description *Message `json:"description,omitempty"`
}

// NewLocationRelationship ...
func NewLocationRelationship(target int) *LocationRelationship {
	t := uint(target)
	return &LocationRelationship{
		Target: t,
	}
}

// WithKind ...
func (l *LocationRelationship) WithKind(kind string) *LocationRelationship {
	l.Kinds = append(l.Kinds, kind)
	return l
}

// WithDescription ...
func (l *LocationRelationship) WithDescription(message *Message) *LocationRelationship {
	l.Description = message
	return l
}
