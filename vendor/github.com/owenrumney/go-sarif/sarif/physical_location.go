package sarif

// PhysicalLocation ...
type PhysicalLocation struct {
	PropertyBag
	ArtifactLocation *ArtifactLocation `json:"artifactLocation,omitempty"`
	Region           *Region           `json:"region,omitempty"`
	ContextRegion    *Region           `json:"contextRegion,omitempty"`
	Address          *Address          `json:"address,omitempty"`
}

// NewPhysicalLocation ...
func NewPhysicalLocation() *PhysicalLocation {
	return &PhysicalLocation{}
}

// WithArtifactLocation ...
func (pl *PhysicalLocation) WithArtifactLocation(artifactLocation *ArtifactLocation) *PhysicalLocation {
	pl.ArtifactLocation = artifactLocation
	return pl
}

// WithRegion ...
func (pl *PhysicalLocation) WithRegion(region *Region) *PhysicalLocation {
	pl.Region = region
	return pl
}
// WithContextRegion ...
func (pl *PhysicalLocation) WithContextRegion(contextRegion *Region) *PhysicalLocation {
	pl.ContextRegion = contextRegion
	return pl
}

// WithAddress ...
func (pl *PhysicalLocation) WithAddress(address *Address) *PhysicalLocation {
	pl.Address = address
	return pl
}
