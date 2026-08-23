package frame

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// VisibleElement allows UI elements to be conditionally hidden, but still present in the model state
type VisibleElement interface {
	IsHidden() bool
}

// TerminalElement allows UI elements to have a lifecycle, where at the end of the lifecycle the element is removed
// from the model state entirely
type TerminalElement interface {
	IsAlive() bool
}

// ImprintableElement is a special case of a TerminalElement, where the element is removed from the model state after a
// printing the model state as a trail behind the current model and removing the element on the next update
type ImprintableElement interface {
	ShouldImprint() bool
}

type Frame struct {
	footer         *bytes.Buffer
	models         []annotatedModel
	windowSize     tea.WindowSizeMsg
	showFooter     bool
	truncateFooter bool
}

type annotatedModel struct {
	model   tea.Model
	expired bool
	hidden  bool
}

func New() *Frame {
	return &Frame{
		footer:         &bytes.Buffer{},
		showFooter:     true,
		truncateFooter: true,
	}
}

func (f Frame) Footer() io.ReadWriter {
	return f.footer
}

func (f *Frame) ShowFooter(set bool) {
	f.showFooter = set
}

func (f *Frame) TruncateFooter(set bool) {
	f.truncateFooter = set
}

func (f *Frame) AppendModel(uiElement tea.Model) {
	f.models = append(f.models, annotatedModel{model: uiElement})
}

func (f Frame) Init() tea.Cmd {
	return nil
}

func (f *Frame) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		f.windowSize = msg
	}

	var cmds []tea.Cmd

	// 1. prune any models that are no longer alive
	// 2. hide/show any models based on the latest state
	// 3. trail any models that are expired, pruning them on the next update
	for i := 0; i < len(f.models); i++ {
		if p, ok := f.models[i].model.(TerminalElement); ok && !p.IsAlive() {
			f.models = append(f.models[:i], f.models[i+1:]...)
			i--
			continue
		}

		if f.models[i].expired {
			f.models = append(f.models[:i], f.models[i+1:]...)
			i--
			continue
		}

		if p, ok := f.models[i].model.(VisibleElement); ok && p.IsHidden() {
			f.models[i].hidden = true
		} else {
			f.models[i].hidden = false
		}

		if p, ok := f.models[i].model.(ImprintableElement); ok && p.ShouldImprint() {
			f.models[i].expired = true

			cmd := tea.Printf("%s", f.models[i].model.View())
			cmds = append(cmds, cmd)
		}
	}

	for i, el := range f.models {
		if el.expired {
			continue
		}
		newEl, cmd := el.model.Update(msg)
		cmds = append(cmds, cmd)
		f.models[i].model = newEl
	}
	return f, tea.Batch(cmds...)
}

func (f Frame) View() string {
	// all UI elements
	var strs []string
	for _, p := range f.models {
		if p.hidden {
			continue
		}
		rendered := p.model.View()
		if len(rendered) > 0 {
			strs = append(strs, rendered)
		}
	}

	str := strings.Join(strs, "\n")

	// log events
	if f.showFooter {
		contents := f.footer.String()
		if f.truncateFooter {
			logLines := strings.Split(contents, "\n")
			logMax := f.windowSize.Height - strings.Count(str, "\n")
			trimLog := len(logLines) - logMax
			if trimLog > 0 && len(logLines) >= trimLog {
				logLines = logLines[trimLog:]
			}
			for _, line := range logLines {
				if len(line) > 0 {
					str += fmt.Sprintf("%s\n", line)
				}
			}
		} else {
			str += contents
		}
	}
	return str
}
