package taskprogress

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/go-progress"
)

type Option func(*Model)

func WithProgress(prog progress.Progressable) Option {
	return func(m *Model) {
		m.progressor = progress.NewGenerator(prog, prog)
	}
}

func WithStager(s progress.Stager) Option {
	return func(m *Model) {
		m.stager = s
	}
}

func WithStagedProgressable(s progress.StagedProgressable) Option {
	return func(m *Model) {
		WithProgress(s)(m)
		WithStager(s)(m)
	}
}

func WithNoStyle() Option {
	return func(m *Model) {
		m.SuccessStyle = lipgloss.NewStyle()
		m.ContextStyle = lipgloss.NewStyle()
		m.FailedStyle = lipgloss.NewStyle()
		m.HintStyle = lipgloss.NewStyle()
		m.TitleStyle = lipgloss.NewStyle()
		m.ProgressBar.FullColor = ""
		m.ProgressBar.EmptyColor = ""
		m.ProgressBar.Full = '|'
		m.ProgressBar.Empty = '-'
	}
}
