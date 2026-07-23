package sarif

// MultiformatMessageString ...
type MultiformatMessageString struct {
	PropertyBag
	Text     *string `json:"text,omitempty"`
	Markdown *string `json:"markdown,omitempty"`
}

// NewMarkdownMultiformatMessageString ...
func NewMarkdownMultiformatMessageString(markdown string) *MultiformatMessageString {
	return &MultiformatMessageString{
		Markdown: &markdown,
	}
}

// NewMultiformatMessageString ...
func NewMultiformatMessageString(text string) *MultiformatMessageString {
	return &MultiformatMessageString{
		Text: &text,
	}
}

// WithMarkdown ...
func (m *MultiformatMessageString) WithMarkdown(markdown string) *MultiformatMessageString {
	m.Markdown = &markdown
	return m
}
