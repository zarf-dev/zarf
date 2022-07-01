package message

import (
	"fmt"

	"github.com/pterm/pterm"
)

type ProgressBar struct {
	progress  *pterm.ProgressbarPrinter
	startText string
}

func NewProgressBar(total int64, format string, a ...any) *ProgressBar {
	var progress *pterm.ProgressbarPrinter
	text := fmt.Sprintf(format, a...)
	if NoProgress {
		Info(text)
	} else {
		progress, _ = pterm.DefaultProgressbar.
			WithTotal(int(total)).
			WithShowCount(false).
			WithTitle(text).
			WithRemoveWhenDone(true).
			Start()
	}

	return &ProgressBar{
		progress:  progress,
		startText: text,
	}
}

func (p *ProgressBar) Update(complete int64, text string) {
	if NoProgress {
		return
	}
	p.progress.UpdateTitle(text)
	chunk := int(complete) - p.progress.Current
	p.progress.Add(chunk)
}

func (p *ProgressBar) Write(data []byte) (int, error) {
	n := len(data)
	if p.progress != nil {
		p.progress.Add(n)
	}
	return n, nil
}

func (p *ProgressBar) Success(text string, a ...any) {
	p.Stop()
	pterm.Success.Printfln(text, a...)
}

func (p *ProgressBar) Stop() {
	if p.progress != nil {
		_, _ = p.progress.Stop()
	}
}

func (p *ProgressBar) Fatalf(err error, format string, a ...any) {
	p.Stop()
	Fatalf(err, format, a...)
}
