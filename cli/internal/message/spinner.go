package message

import (
	"fmt"

	"github.com/pterm/pterm"
)

type Spinner struct {
	spinner   *pterm.SpinnerPrinter
	startText string
}

func NewProgresSpinner(format string, a ...interface{}) *Spinner {
	text := fmt.Sprintf(format, a...)
	spinner, _ := pterm.DefaultSpinner.
		WithRemoveWhenDone(false).
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
	_ = p.spinner.Stop()
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
