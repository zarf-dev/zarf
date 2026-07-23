package httpx

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/henvic/httpretty"

	"github.com/gpustack/gguf-parser-go/util/json"
)

var _ httpretty.Formatter = (*JSONFormatter)(nil)

// JSONFormatter is copied from httpretty.JSONFormatter,
// but use our own json package.
type JSONFormatter struct{}

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

// Match JSON media type.
func (j *JSONFormatter) Match(mediatype string) bool {
	return jsonTypeRE.MatchString(mediatype)
}

// Format JSON content.
func (j *JSONFormatter) Format(w io.Writer, src []byte) error {
	if !json.Valid(src) {
		// We want to get the error of json.checkValid, not unmarshal it.
		// The happy path has been optimized, maybe prematurely.
		if err := json.Unmarshal(src, &json.RawMessage{}); err != nil {
			return err
		}
	}
	// Avoiding allocation as we use *bytes.Buffer to store the formatted body before printing
	dst, ok := w.(*bytes.Buffer)
	if !ok {
		// Mitigating panic to avoid upsetting anyone who uses this directly
		return errors.New("underlying writer for JSONFormatter must be *bytes.Buffer")
	}
	return json.Indent(dst, src, "", "    ")
}

type RoundTripperChain struct {
	Do   func(req *http.Request) error
	Next http.RoundTripper
}

func (c RoundTripperChain) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.Do != nil {
		if err := c.Do(req); err != nil {
			return nil, err
		}
	}
	if c.Next != nil {
		return c.Next.RoundTrip(req)
	}
	return nil, nil
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
