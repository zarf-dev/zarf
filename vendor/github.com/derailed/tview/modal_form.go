package tview

import (
	"github.com/derailed/tcell/v2"
)

// ModalForm implements a modal window with a custom form.
type ModalForm struct {
	*Modal
}

// NewModalForm implements a modal that can take in a custom form.
func NewModalForm(title string, form *Form) *ModalForm {
	m := ModalForm{NewModal()}
	m.form = form
	m.form.SetBackgroundColor(Styles.ContrastBackgroundColor).SetBorderPadding(0, 0, 0, 0)
	m.form.SetCancelFunc(func() {
		if m.done != nil {
			m.done(-1, "")
		}
	})
	m.frame = NewFrame(m.form).SetBorders(0, 0, 1, 0, 0, 0)
	m.frame.SetBorder(true).
		SetBackgroundColor(Styles.ContrastBackgroundColor).
		SetBorderPadding(1, 1, 1, 1)
	m.frame.SetTitle(title)
	m.frame.SetTitleColor(tcell.ColorAqua)
	m.focus = m

	return &m
}

// Draw draws this primitive onto the screen.
func (m *ModalForm) Draw(screen tcell.Screen) {
	// Calculate the width of this modal.
	buttonsWidth := 0
	for _, button := range m.form.buttons {
		buttonsWidth += TaggedStringWidth(button.label) + 4 + 2
	}
	buttonsWidth -= 2
	screenWidth, screenHeight := screen.Size()
	width := screenWidth / 3
	if width < buttonsWidth {
		width = buttonsWidth
	}
	// width is now without the box border.

	// Reset the text and find out how wide it is.
	m.frame.Clear()
	lines := WordWrap(m.text, width)
	for _, line := range lines {
		m.frame.AddText(line, true, AlignCenter, m.textColor)
	}

	// Set the modal's position and size.
	height := len(lines) + len(m.form.items) + len(m.form.buttons) + 5
	width += 4
	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	m.SetRect(x, y, width, height)

	// Draw the frame.
	m.frame.SetRect(x, y, width, height)
	m.frame.Draw(screen)
}
