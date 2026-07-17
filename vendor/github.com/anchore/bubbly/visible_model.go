package bubbly

import tea "github.com/charmbracelet/bubbletea"

type VisibleModel interface {
	IsVisible() bool
	tea.Model
}
