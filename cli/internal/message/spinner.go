package message

import (
	"fmt"
	"github.com/pterm/pterm"
)

type Spinner struct {
	spinner   *pterm.SpinnerPrinter
	startText string
}

func NewProgressSpinner(format string, a ...interface{}) *Spinner {
	text := fmt.Sprintf(format, a...)
	spinner, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(false).
		// Src: https://github.com/gernest/wow/blob/master/spin/spinners.go#L335
		WithSequence(`  ⠋ `, `  ⠙ `, `  ⠹ `, `  ⠸ `, `  ⠼ `, `  ⠴ `, `  ⠦ `, `  ⠧ `, `  ⠇ `, `  ⠏ `).
		Start(text)

	return &Spinner{
		spinner:   spinner,
		startText: text,
	}
}

func (p *Spinner) Write(text []byte) (int, error) {
	Debug(string(text))
	return len(text), nil
}

func (p *Spinner) Updatef(format string, a ...interface{}) {
	text := fmt.Sprintf(format, a...)
	p.spinner.UpdateText(text)
}

func (p *Spinner) Debugf(format string, a ...interface{}) {
	if logLevel >= DebugLevel {
		text := fmt.Sprintf("Debug: "+format, a...)
		p.spinner.UpdateText(text)
	}
}

func (p *Spinner) Stop() {
	if p.spinner.IsActive {
		// Only stop if not stopped to avoid extra line break injections in the CLI
		_ = p.spinner.Stop()
		Debug("Possible spinner leak detected")
	}
}

func (p *Spinner) Success() {
	p.Successf(p.startText)
}

func (p *Spinner) Successf(format string, a ...interface{}) {
	text := fmt.Sprintf(format, a...)
	p.spinner.Success(text)
}

func (p *Spinner) Warnf(format string, a ...interface{}) {
	text := fmt.Sprintf(format, a...)
	p.spinner.Warning(text)
}

func (p *Spinner) Errorf(err error, format string, a ...interface{}) {
	p.Warnf(format, a...)
	Debug(err)
}

func (p *Spinner) Fatalf(err error, format string, a ...interface{}) {
	p.spinner.Fail(p.startText)
	Fatalf(err, format, a...)
}
