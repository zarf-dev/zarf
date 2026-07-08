package sarif

// Location ...
type Location struct {
	PropertyBag
	Id               *uint                   `json:"id,omitempty"`
	PhysicalLocation *PhysicalLocation       `json:"physicalLocation,omitempty"`
	LogicalLocations []*LogicalLocation      `json:"logicalLocations,omitempty"`
	Message          *Message                `json:"message,omitempty"`
	Annotations      []*Region               `json:"annotations,omitempty"`
	Relationships    []*LocationRelationship `json:"relationships,omitempty"`
}

// NewLocation ...
func NewLocation() *Location {
	return &Location{}
}

// NewLocationWithPhysicalLocation ...
func NewLocationWithPhysicalLocation(physicalLocation *PhysicalLocation) *Location {
	return NewLocation().WithPhysicalLocation(physicalLocation)
}

// WithId ...
func (l *Location) WithId(id int) *Location {
	i := uint(id)
	l.Id = &i
	return l
}

// WithPhysicalLocation ...
func (l *Location) WithPhysicalLocation(physicalLocation *PhysicalLocation) *Location {
	l.PhysicalLocation = physicalLocation
	return l
}

// WithMessage ...
func (l *Location) WithMessage(message *Message) *Location {
	l.Message = message
	return l
}

// WithAnnotation ...
func (l *Location) WithAnnotation(region *Region) *Location {
	l.Annotations = append(l.Annotations, region)
	return l
}

// WithRelationship ...
func (l *Location) WithRelationship(locationRelationship *LocationRelationship) *Location {
	l.Relationships = append(l.Relationships, locationRelationship)
	return l
}
