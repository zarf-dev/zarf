package sarif

// ArtifactChange ...
type ArtifactChange struct {
	PropertyBag
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Replacements     []*Replacement   `json:"replacements"` //required
}

// NewArtifactChange ...
func NewArtifactChange(artifactLocation *ArtifactLocation) *ArtifactChange {
	return &ArtifactChange{
		ArtifactLocation: *artifactLocation,
	}
}

// WithReplacement ...
func (a *ArtifactChange) WithReplacement(replacement *Replacement) *ArtifactChange {
	a.Replacements = append(a.Replacements, replacement)
	return a
}
