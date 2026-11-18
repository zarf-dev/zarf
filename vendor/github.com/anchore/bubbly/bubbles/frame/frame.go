package frame

import (
	"bytes"
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
	// all UI elements and log events are collected as rows and joined once, so every row (including the first
	// footer row) starts on its own line and the row count matches what the renderer draws
	var lines []string
	for _, p := range f.models {
		if p.hidden {
			continue
		}
		rendered := p.model.View()
		if len(rendered) > 0 {
			lines = append(lines, strings.Split(rendered, "\n")...)
		}
	}

	// log events
	if f.showFooter {
		var logLines []string
		for _, line := range strings.Split(f.footer.String(), "\n") {
			if len(line) > 0 {
				logLines = append(logLines, line)
			}
		}
		if f.truncateFooter {
			// keep only the most recent log lines that fit below the models (and the trailing blank row). Before the
			// first WindowSizeMsg the height is 0, so nothing is shown.
			logMax := max(0, f.windowSize.Height-len(lines)-1)
			if len(logLines) > logMax {
				logLines = logLines[len(logLines)-logMax:]
			}
		}
		lines = append(lines, logLines...)
	}
	if len(lines) == 0 {
		return ""
	}
	// every row is newline terminated so the renderer's last row is blank. On exit, bubbletea erases the row the
	// cursor is on, which would otherwise wipe out the last model or log line.
	return strings.Join(lines, "\n") + "\n"
}
