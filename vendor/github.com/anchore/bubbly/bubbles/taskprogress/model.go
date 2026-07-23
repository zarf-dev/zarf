package taskprogress

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
	progressBubble "github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/bubbly"
)

const (
	checkMark = "✔"
	xMark     = "✘"
)

var _ bubbly.VisibleModel = (*Model)(nil)

type Model struct {
	// ui components (view models)
	Spinner     spinner.Model
	ProgressBar progressBubble.Model
	title       string
	hints       []string
	context     []string

	// enums for view model
	TitleOptions Title
	Hints        []string
	Context      []string

	// state that drives view ui components
	progress   *progress.Progress
	progressor progress.Progressor
	stager     progress.Stager
	WindowSize tea.WindowSizeMsg
	completed  bool
	err        error

	UpdateDuration        time.Duration
	HideProgressOnSuccess bool
	HideStageOnSuccess    bool
	HideOnSuccess         bool

	TitleStyle lipgloss.Style
	// TitlePendingStyle lipgloss.Style
	HintStyle    lipgloss.Style
	ContextStyle lipgloss.Style
	SuccessStyle lipgloss.Style
	FailedStyle  lipgloss.Style
	TitleWidth   int
	HintEndCaps  []string

	id       int
	sequence int

	// coordinate if there are any live components on the UI
	done func()
}

// New returns a model with default values.
func New(wg *sync.WaitGroup, opts ...Option) Model {
	wg.Add(1)
	once := sync.Once{}
	done := func() {
		once.Do(wg.Done)
	}
	spin := spinner.New()

	// matches the same spinner as syft/grype
	spin.Spinner = spinner.Spinner{
		Frames: strings.Split("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏", ""),
		FPS:    150 * time.Millisecond,
	}
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("13")) // 13 = high intentity magenta (ANSI 16 bit color code)

	prog := progressBubble.New(
		progressBubble.WithoutPercentage(),
		progressBubble.WithWidth(20),
	)
	// matches the same progress feel as syft/grype
	prog.Full = '━'
	prog.Empty = '━'
	// TODO: make responsive to light/dark themes
	prog.EmptyColor = "#777777"
	prog.FullColor = "#fcba03"

	m := Model{
		Spinner:        spin,
		ProgressBar:    prog,
		UpdateDuration: 250 * time.Millisecond,
		id:             nextID(),
		done:           done,

		TitleStyle: lipgloss.NewStyle().Bold(true),
		//TitlePendingStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
		//	Light: "#555555",
		//	Dark:  "#AAAAAA",
		// }),
		ContextStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")),
		HintStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")),
		SuccessStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // 10 = high intensity green (ANSI 16 bit color code)
		FailedStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),  // 9 = high intensity red (ANSI 16 bit color code)
		TitleWidth:   40,
		HintEndCaps:  []string{"[", "]"},
	}

	for _, opt := range opts {
		opt(&m)
	}
	return m
}

func (m Model) hintCap(end bool) string {
	l := len(m.HintEndCaps)
	if l == 0 {
		return ""
	}
	if end {
		return m.HintEndCaps[l-1]
	}
	return m.HintEndCaps[0]
}

// Init is the command that effectively starts the continuous update loop.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		// this is the periodic update of state information
		func() tea.Msg {
			return TickMsg{
				// The time at which the tick occurred.
				Time: time.Now(),

				// The ID of the spinner that this message belongs to. This can be
				// helpful when routing messages, however bear in mind that spinners
				// will ignore messages that don't contain ID by default.
				ID: m.id,

				Sequence: m.sequence,
			}
		},
		m.Spinner.Tick,
		m.ProgressBar.Init(),
	}

	return tea.Batch(
		cmds...,
	)
}

// ID returns the spinner's unique ID.
func (m Model) ID() int {
	return m.id
}

// Sequence returns the spinner's current sequence number.
func (m Model) Sequence() int {
	return m.sequence
}

// Update is the Tea update function.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.WindowSize = msg
		return m, nil

	case TickMsg:
		tickCmd := m.handleTick(msg)
		if tickCmd == nil {
			// this tick is not meant for us
			return m, nil
		}

		// this tick is meant for us... do an update!
		var progCmd tea.Cmd

		title := m.TitleOptions.Default
		var prog *progress.Progress
		if m.progressor != nil {
			c := m.progressor.Progress()
			title = m.TitleOptions.Title(c)
			if c.Size() > 0 {
				prog = &c
				ratio := c.Ratio()
				if m.ProgressBar.Percent() != ratio {
					progCmd = m.ProgressBar.SetPercent(ratio)
				}
			}
			m.completed = c.Complete()
			if c.Error() != nil && !errors.Is(c.Error(), progress.ErrCompleted) {
				m.err = c.Error()
			}
		}
		m.title = title
		m.progress = prog

		if m.stager != nil {
			stage := m.stager.Stage()
			if stage != "" {
				// TODO: how to deal with stages that have custom stats from the results of commands?
				// TODO: list is awkward both in usage and display
				m.hints = append([]string{stage}, m.Hints...)
			} else {
				m.hints = m.Hints
			}
		}

		// TODO: rethink this
		m.context = m.Context

		return m, tea.Batch(tickCmd, progCmd)

	case progressBubble.FrameMsg:
		progModel, progCmd := m.ProgressBar.Update(msg)
		m.ProgressBar = progModel.(progressBubble.Model)
		return m, progCmd

	case spinner.TickMsg:
		spinModel, spinCmd := m.Spinner.Update(msg)
		m.Spinner = spinModel
		return m, spinCmd

	default:
		return m, nil
	}
}

func (m Model) IsVisible() bool {
	isDoneAndHidden := m.completed && m.HideOnSuccess
	if isDoneAndHidden {
		// it might be that the consumer will not invoke View() again based on
		// this response, in which case we need to ensure that the done() function
		// in invoked to release resources
		m.done()
	}

	return !(isDoneAndHidden)
}

// View renders the model's view.
func (m Model) View() string {
	if !m.IsVisible() {
		m.done()
		return ""
	}
	beforeProgress := " "
	if m.completed {
		if m.err != nil {
			beforeProgress += m.FailedStyle.Render(xMark) + " "
		} else {
			beforeProgress += m.SuccessStyle.Render(checkMark) + " "
		}
	} else {
		beforeProgress += m.Spinner.View() + " "
	}

	if m.title != "" {
		beforeProgress += m.TitleStyle.Width(m.TitleWidth).Align(lipgloss.Left).Render(m.title) + "  "
	}

	progressBar := ""
	var progressBarWidth int
	showProgress := m.progress != nil && (!m.completed || (m.completed && !m.HideProgressOnSuccess && m.err == nil))
	if showProgress {
		progressBar += m.ProgressBar.View() + "  "
		progressBarWidth = m.ProgressBar.Width + 2
	}

	afterProgress := ""

	showStage := (!m.completed || (m.completed && !m.HideStageOnSuccess)) && len(m.hints) > 0
	if showStage {
		var hints []string
		for _, h := range m.hints {
			hints = append(hints, fmt.Sprintf("%s%s%s", m.hintCap(false), h, m.hintCap(true)))
		}
		hintStr := strings.Join(hints, " ")
		afterProgress += m.HintStyle.Render(hintStr) + "  "
	}

	if len(m.context) > 0 {
		width := m.WindowSize.Width - (len(stripansi.Strip(beforeProgress+afterProgress)) + progressBarWidth)
		afterProgress += m.ContextStyle.Width(width).Align(lipgloss.Right).Render(strings.Join(m.context, " "))
	}

	if m.completed {
		defer m.done()
	}

	// force overflow to be ignored
	return lipgloss.NewStyle().Inline(true).Render(beforeProgress + progressBar + afterProgress)
}

func (m Model) queueNextTick(id, sequence int) tea.Cmd {
	return tea.Tick(m.UpdateDuration, func(t time.Time) tea.Msg {
		return TickMsg{
			Time:     t,
			ID:       id,
			Sequence: sequence,
		}
	})
}

func (m *Model) handleTick(msg TickMsg) tea.Cmd {
	// If an ID is set, and the ID doesn't belong to this spinner, reject
	// the message.
	if msg.ID > 0 && msg.ID != m.id {
		return nil
	}

	// If a sequence is set, and it's not the one we expect, reject the message.
	// This prevents the spinner from receiving too many messages and
	// thus spinning too fast.
	if msg.Sequence > 0 && msg.Sequence != m.sequence {
		return nil
	}

	m.sequence++

	// we should still respond to stage changes and window size events
	// if m.completed {
	//	return nil
	//}

	return m.queueNextTick(m.id, m.sequence)
}
