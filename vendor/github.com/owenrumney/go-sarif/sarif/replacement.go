package sarif

// Replacement ...
type Replacement struct {
	PropertyBag
	DeletedRegion   Region           `json:"deletedRegion"`
	InsertedContent *ArtifactContent `json:"insertedContent,omitempty"`
}

// NewReplacement ...
func NewReplacement(region *Region) *Replacement {
	return &Replacement{
		DeletedRegion: *region,
	}
}

// WithInsertedContent ...
func (r *Replacement) WithInsertedContent(artifactContent *ArtifactContent) *Replacement {
	r.InsertedContent = artifactContent
	return r
}
