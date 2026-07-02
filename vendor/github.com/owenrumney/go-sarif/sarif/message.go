package sarif

// Message ...
type Message struct { // https://docs.oasis-open.org/sarif/sarif/v2.1.0/csprd01/sarif-v2.1.0-csprd01.html#_Toc10540897
	PropertyBag
	Text      *string  `json:"text,omitempty"`
	Markdown  *string  `json:"markdown,omitempty"`
	Id        *string  `json:"id,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
}

// NewMessage ...
func NewMessage() *Message {
	return &Message{}
}

// NewTextMessage ...
func NewTextMessage(text string) *Message {
	return NewMessage().WithText(text)
}

// NewMarkdownMessage ...
func NewMarkdownMessage(markdown string) *Message {
	return NewMessage().WithMarkdown(markdown)
}

// WithText ...
func (m *Message) WithText(text string) *Message {
	m.Text = &text
	return m
}

// WithMarkdown ...
func (m *Message) WithMarkdown(markdown string) *Message {
	m.Markdown = &markdown
	return m
}

// WithId ...
func (m *Message) WithId(id string) *Message {
	m.Id = &id
	return m
}

// WithArgument ...
func (m *Message) WithArgument(argument string) *Message {
	m.Arguments = append(m.Arguments, argument)
	return m
}
