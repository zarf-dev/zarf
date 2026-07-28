package sarif

// Fix ...
type Fix struct {
	PropertyBag
	Description     *Message          `json:"description,omitempty"`
	ArtifactChanges []*ArtifactChange `json:"artifactChanges"` //	required
}

// NewFix ...
func NewFix() *Fix {
	return &Fix{}
}

// WithDescription ...
func (f *Fix) WithDescription(message *Message) *Fix {
	f.Description = message
	return f
}

// WithArtifactChange ...
func (f *Fix) WithArtifactChange(ac *ArtifactChange) *Fix {
	f.ArtifactChanges = append(f.ArtifactChanges, ac)
	return f
}
