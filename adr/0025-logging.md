# 25. Logging

Date: 2024-06-06

## Status

Proposed

## Context

Zarf is currently using an in-house built logging solution which in turn depends on pterm. This solution is used to output information to the end user who is using the Zarf CLI. This output is for the most part formatted with the purpose of CLI user experience. Logging function calls are done both in the CLI specific code as well as in the packages. The logging is implemented in such a way that different levels exist with different impacts and log destinations. A common pattern that is used is to call `message.Fatal` whenever an error occurs. This will output the message to STDERR while writing the actual error message to a debug log and exiting the program. Exiting the program in this manner makes unit testing difficult while also skipping proper context handling and skipping any clean-up that was intended to run before exiting the program. Some logging components like the progress bar and spinner are accessed through a shared global state that is not thread safe. The coupling to logging becomes complicated as disabling the progress bar is a challenge while multi threading these functions which access the global state resulting in complicated access patterns.

## Decision

I am proposing to completely refactor the logging functionality to follow a more standardized format using the newish slog interfaces. On top of that we would refactor the current internationalization by converting them to standardized errors.

* Replace the message package with a slog interface.
* Replace the lang package with static errors.
* Remove any use of `message.Fatal` and instead return errors.
* Refactor use of `message.WarnErr` to either return error or to log the error.
* Refactor existing functions that print formatted outputs to defer the printing to the CLI.
* Define a new interface for spinner and progress bar that is passed in as a function parameter.

The message package currently exports the following functions which should be replaced by its logr counterpart.

| Function | Replacement | Comment |
| --- | --- | --- |
| ZarfCommand | Info | Just formats a slice as a command. |
| Command | Info | Outputs a command with a prefix and style. |
| Debug | Debug | |
| Debugf | Debug | |
| ErrorWebf | N/A | Not used anywhere. |
| Warn | Warn | |
| Warnf | Warn | |
| WarnErr | Warn | |
| WarnErrf | Warn | |
| Fatal | N/A | Should not be used. |
| Fatalf | N/A | Should not be used. |
| Info | Info | |
| Infof | Info | |
| Success | Info | Seems to be a info log with a checkmark prefix. |
| Successf | Info | Seems to be a info log with a checkmark prefix. |
| Question | ? | Open question how to resolve this. |
| Questionf | ? | Open question how to resolve this. |
| Note | Info | Should just be an info or maybe a debug log. |
| Notef | Info | Should just be an info or maybe a debug log. |
| Title | ? | Not sure how this should be replaced as it formats with separator. |
| HeaderInfof | ? | |
| HorizontalRule | ? | |
| JSONValue | N/A | Should be replaced with a marshal. |
| Paragraph | ? | |
| Paragraphn | ? | |
| PrintDiff | ? | |
| Table | ? | Need to come up with a good syntax for functions to return output that can print as a table. |
| ColorWrap | ? | Should this be used? | 
| PrintConnectStringTable | N/A | This logic should not exist in the message package. |
| PrintCredentialTable | N/A | This logic should not exist in the message package. |
| PrintComponentCredential | N/A | This logic should not exist in the message package. |
| PrintCredentialUpdates | N/A | This logic should not exist in the message package. |
| Spinner | Interface | New Spinner interface. |
| ProgressBar | Interface | New progress bar interface. |

The majority of simple logging changes should be possible with little signature changes. Replacing the existing output with a slog interface would allow other implementations to be used. A challenge initially may be the changes to table output formatting to make it work properly. This change will require some refactoring of existing code. A requirement for the changes is that they have to improve the UX for users looking at log files. As I understand the present day the spinners will cause a new line everytime they update, resulting in a lot of bloat. A goal should be to make sure that this does not happen in the future.

Spinners and progress bars however are a bit more challenging. They need to be refactored so that they are no longer instantiated outside of the CLI code. They should instead be passed as a function parameter to each function that needs them. A good example of a project that solves the problem in a similar manner is Kind. In Kind they create a [status object from the logger](https://github.com/kubernetes-sigs/kind/blob/7799e72306db315ea4f4b1cac90ff68404da4f28/pkg/internal/cli/status.go#L39) which they then [pass to where it is needed](https://github.com/kubernetes-sigs/kind/blob/7799e72306db315ea4f4b1cac90ff68404da4f28/pkg/cluster/internal/create/create.go#L133). Doing so results in a single status object created which is reused where ever it is needed. A lot of inspiration can be take from how Kind deals with CLI output. While they use klog instead of slog there are a lot of similarities. They have for example a check if the output is in a terminal or not, and will disable the spinner accordingly. 

Here is a suggestion for how a thread safe spinner could be implemented with a shared logger. This also allows for parallel spinners and progress bars.

```golang
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	h := NewPtermHandler(os.Stderr)
	log := slog.New(h)

	log.Info("before")

	spinner := NewSpinner(log)
	spinner.Update("Running some job")
	log.Info("after")
	time.Sleep(1 * time.Second)
	spinner.Update("Doing some update")

	time.Sleep(2 * time.Second)

	spinner.Succeed()

	time.Sleep(2 * time.Second)

	return nil
}

type PtermHandler struct {
	printer *pterm.MultiPrinter
	attrs   []slog.Attr
	group   string
}

func NewPtermHandler(w io.Writer) *PtermHandler {
	printer, _ := pterm.DefaultMultiPrinter.WithWriter(w).Start()
	return &PtermHandler{
		printer: printer,
	}
}

func (h *PtermHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *PtermHandler) Handle(ctx context.Context, r slog.Record) error {
	l := fmt.Sprintf("%s: %s\n", r.Level, r.Message)
	_, err := h.printer.NewWriter().Write([]byte(l))
	return err
}

func (h *PtermHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &PtermHandler{
		printer: h.printer,
		attrs:   append(h.attrs, attrs...),
		group:   h.group,
	}
}

func (h *PtermHandler) WithGroup(name string) slog.Handler {
	return &PtermHandler{
		printer: h.printer,
		attrs:   h.attrs,
		group:   name,
	}
}

type Spinner struct {
	sequence    []string
	mx          sync.Mutex
	log         *slog.Logger
	printer     *pterm.MultiPrinter
	firstStatus string
	status      string
	spinner     *pterm.SpinnerPrinter
}

func NewSpinner(log *slog.Logger) *Spinner {
	h, ok := log.Handler().(*PtermHandler)
	if !ok {
		return &Spinner{
			log: log,
		}
	}
	return &Spinner{
		sequence: []string{`  ⠋ `, `  ⠙ `, `  ⠹ `, `  ⠸ `, `  ⠼ `, `  ⠴ `, `  ⠦ `, `  ⠧ `, `  ⠇ `, `  ⠏ `},
		log:      log,
		printer:  h.printer,
	}
}

func (s *Spinner) Update(status string) {
	s.mx.Lock()
	defer s.mx.Unlock()

	// Do not update if status is the same.
	if s.status == status {
		return
	}
	if s.firstStatus == "" {
		s.firstStatus = status
	}
	s.status = status

	// If no printer we log normally.
	if s.printer == nil {
		s.log.Info(status)
		return
	}

	// Create or update the spinner.
	if s.spinner == nil {
		spinner, _ := pterm.DefaultSpinner.WithWriter(s.printer.NewWriter()).WithSequence(s.sequence...).Start(status)
		s.spinner = spinner
		return
	}
	s.spinner.UpdateText(status)
}

func (s *Spinner) Fail() {
	s.mx.Lock()
	defer s.mx.Lock()

	if s.printer == nil {
		return
	}
	s.spinner.Fail(s.firstStatus)
}

func (s *Spinner) Succeed() {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.printer == nil {
		return
	}
	s.spinner.Success(s.firstStatus)
}
```

The work will most likely have to be split into a couple of steps.

1. Remove any use of message fatal.
2. Refactor table printing functions.
3. Replace message logging with a structured logger.
4. Replace spinner and progress bars.

## Consequences

Refactoring the message package would make importing Zarf packages as a library simpler. It would also simplify any unit testing and debugging efforts by using predictable errors. Additionally it should allow us to enable parallel testing where we have disabled it currently.

While not intended it may have some user facing change if we chose to change the format of the log output slightly. While that may not be the intention currently it may become so in the future.
