package sarif

// ArtifactLocation ...
type ArtifactLocation struct { // https://docs.oasis-open.org/sarif/sarif/v2.1.0/csprd01/sarif-v2.1.0-csprd01.html#_Toc10540865
	PropertyBag
	URI         *string  `json:"uri,omitempty"`
	URIBaseId   *string  `json:"uriBaseId,omitempty"`
	Index       *uint    `json:"index,omitempty"`
	Description *Message `json:"description,omitempty"`
}

// NewArtifactLocation ...
func NewArtifactLocation() *ArtifactLocation {
	return &ArtifactLocation{}
}

// NewSimpleArtifactLocation ...
func NewSimpleArtifactLocation(uri string) *ArtifactLocation {
	return NewArtifactLocation().WithUri(uri)
}

// WithUri ...
func (a *ArtifactLocation) WithUri(uri string) *ArtifactLocation {
	a.URI = &uri
	return a
}

// WithUriBaseId ...
func (a *ArtifactLocation) WithUriBaseId(uriBaseId string) *ArtifactLocation {
	a.URIBaseId = &uriBaseId
	return a
}

// WithIndex ...
func (a *ArtifactLocation) WithIndex(index int) *ArtifactLocation {
	i := uint(index)
	a.Index = &i
	return a
}

// WithDescription ...
func (a *ArtifactLocation) WithDescription(message *Message) *ArtifactLocation {
	a.Description = message
	return a
}
