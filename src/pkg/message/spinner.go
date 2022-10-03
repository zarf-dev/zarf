package message

import (
	"fmt"

	"github.com/pterm/pterm"
)

var activeSpinner *Spinner

type Spinner struct {
	spinner   *pterm.SpinnerPrinter
	startText string
}

func NewProgressSpinner(format string, a ...any) *Spinner {
	if activeSpinner != nil {
		Debug("Active spinner already exists")
		return activeSpinner
	}

	var spinner *pterm.SpinnerPrinter
	text := fmt.Sprintf(format, a...)
	if NoProgress {
		Info(text)
	} else {
		spinner, _ = pterm.DefaultSpinner.
			WithRemoveWhenDone(false).
			// Src: https://github.com/gernest/wow/blob/master/spin/spinners.go#L335
			WithSequence(`  ⠋ `, `  ⠙ `, `  ⠹ `, `  ⠸ `, `  ⠼ `, `  ⠴ `, `  ⠦ `, `  ⠧ `, `  ⠇ `, `  ⠏ `).
			Start(text)
	}

	activeSpinner = &Spinner{
		spinner:   spinner,
		startText: text,
	}

	return activeSpinner
}

func (p *Spinner) Write(text []byte) (int, error) {
	size := len(text)
	if NoProgress {
		return size, nil
	}
	Debug(string(text))
	return len(text), nil
}

func (p *Spinner) Updatef(format string, a ...any) {
	if NoProgress {
		return
	}

	text := fmt.Sprintf(format, a...)
	p.spinner.UpdateText(text)
}

func (p *Spinner) Debugf(format string, a ...any) {
	if logLevel >= DebugLevel {
		text := fmt.Sprintf("Debug: "+format, a)
		if NoProgress {
			Debug(text)
		} else {
			p.spinner.UpdateText(text)
		}
	}
}

func (p *Spinner) Stop() {
	if p.spinner != nil && p.spinner.IsActive {
		_ = p.spinner.Stop()
	}
	activeSpinner = nil
}

func (p *Spinner) Success() {
	p.Successf(p.startText)
}

func (p *Spinner) Successf(format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	if p.spinner != nil {
		p.spinner.Success(text)
		activeSpinner = nil
	} else {
		Info(text)
	}
}

func (p *Spinner) Warnf(format string, a ...any) {
	text := fmt.Sprintf(format, a...)
	if p.spinner != nil {
		p.spinner.Warning(text)
	} else {
		Warn(text)
	}
}

func (p *Spinner) Errorf(err error, format string, a ...any) {
	p.Warnf(format, a...)
	Debug(err)
}

func (p *Spinner) Fatal(err error) {
	p.Fatalf(err, p.startText)
}

func (p *Spinner) Fatalf(err error, format string, a ...any) {
	if p.spinner != nil {
		p.spinner.RemoveWhenDone = true
		_ = p.spinner.Stop()
		activeSpinner = nil
	}
	Fatalf(err, format, a...)
}
