package sarif

// ArtifactContent ...
type ArtifactContent struct { // https://docs.oasis-open.org/sarif/sarif/v2.1.0/csprd01/sarif-v2.1.0-csprd01.html#_Toc10540860
	PropertyBag
	Text     *string                   `json:"text,omitempty"`
	Binary   *string                   `json:"binary,omitempty"`
	Rendered *MultiformatMessageString `json:"rendered,omitempty"`
}

// NewArtifactContent ...
func NewArtifactContent() *ArtifactContent {
	return &ArtifactContent{}
}

// WithText ...
func (a *ArtifactContent) WithText(text string) *ArtifactContent {
	a.Text = &text
	return a
}

// WithBinary ...
func (a *ArtifactContent) WithBinary(binary string) *ArtifactContent {
	a.Binary = &binary
	return a
}

// WithRendered ...
func (a *ArtifactContent) WithRendered(mms *MultiformatMessageString) *ArtifactContent {
	a.Rendered = mms
	return a
}
