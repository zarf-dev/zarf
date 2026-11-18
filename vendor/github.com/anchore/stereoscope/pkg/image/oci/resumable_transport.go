package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/anchore/stereoscope/internal/log"
)

const (
	// maxConsecutiveStalls is how many attempts in a row may fail to deliver minResumeProgress
	// before the read is abandoned. it is also what bounds a server whose spans are too small to be
	// worth a round trip, since no number of those replenishes anything.
	maxConsecutiveStalls = 5

	// maxTotalResumes bounds the reopens a dropped connection may cost one body, so a path dropping
	// every few kilobytes fails rather than reconnecting indefinitely. one more is earned per
	// resumeCreditSize delivered, so a long transfer is not held to the same count as a short one.
	// a span ending where its Content-Range declared is not a drop and does not spend this; those
	// are bounded by the stall budget above and by the length of the blob.
	maxTotalResumes  = 20
	resumeCreditSize = 64 * 1024 * 1024

	// minResumeProgress is how much must arrive to replenish the stall budget. any n > 0 is not
	// enough, since one byte per connection would replenish it for ever. the surplus above it is
	// discarded rather than banked, so each replenishment is earned afresh.
	minResumeProgress = 64 * 1024

	// idleTimeout bounds how long a reopen may take to get a response, re-authenticating included,
	// and how long a read may wait for its next bytes. a slow body is never on a clock, only a silent
	// one. it is generous so that a proxy pausing to fill from upstream is not taken for a drop.
	idleTimeout = 2 * time.Minute

	// stallBackoff is multiplied by the consecutive stall count. the first attempt after a drop
	// goes out at once.
	stallBackoff = time.Second

	// statusClientClosedRequest is the non-standard 499 some proxies answer with.
	statusClientClosedRequest = 499

	// maxErrorText caps how much of a server-influenced message reaches a log line or an error.
	maxErrorText = 200
)

// errResumeRefused marks a reopen another attempt cannot mend. anything else spends a stall.
var errResumeRefused = errors.New("the server refused to resume")

// resumableTransport wraps a RoundTripper so that a registry blob whose body fails partway through
// is resumed with a Range request from the byte already delivered. go-containerregistry retries
// whole requests only, so without this a dropped connection discards however much of the layer had
// already arrived.
//
// it sits below go-containerregistry's verify.ReadCloser, so the digest and size of every
// reassembled blob are still checked end to end. reopens go back to the registry URL through
// client, which authenticates and follows redirects, so a long transfer gets a fresh token and a
// freshly signed CDN URL rather than ones that lapsed along the way.
type resumableTransport struct {
	base http.RoundTripper
	// clientFor returns what reopens go through, built on first use since building it pings the registry
	clientFor    reopenClientFunc
	minProgress  int64
	backoff      time.Duration
	idleTimeout  time.Duration
	maxStalls    int
	maxResumes   int
	resumeCredit int64
}

// reopenClientFunc returns the client a resumed blob request is reissued through.
type reopenClientFunc func(context.Context) (*http.Client, error)

func newResumableTransport(base http.RoundTripper, clientFor reopenClientFunc) *resumableTransport {
	return &resumableTransport{
		base:         base,
		clientFor:    clientFor,
		minProgress:  minResumeProgress,
		backoff:      stallBackoff,
		idleTimeout:  idleTimeout,
		maxStalls:    maxConsecutiveStalls,
		maxResumes:   maxTotalResumes,
		resumeCredit: resumeCreditSize,
	}
}

func (t *resumableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || !t.shouldResume(req, resp) {
		return resp, err
	}

	resp.Body = &resumableBody{
		t:       t,
		body:    watchIdle(resp.Body, t.idleTimeout),
		req:     req,
		total:   resp.ContentLength,
		spanEnd: resp.ContentLength,
		done:    make(chan struct{}),
	}

	return resp, nil
}

// shouldResume reports whether a response body is worth wrapping: a plain, unranged GET for a
// registry blob that returned a body of known length in the identity coding.
//
// only blobs, since only blobs are digest-verified above this transport. the length is the only way
// to tell a truncated stream from a complete one, and offsets are over wire bytes, so an encoded
// stream cannot be resumed (net/http sets ContentLength to -1 on one it decodes itself).
func (t *resumableTransport) shouldResume(req *http.Request, resp *http.Response) bool {
	return req.Method == http.MethodGet &&
		isBlobRequest(req) &&
		// a cloned request shares the body reader, so a reissued one would resend a consumed body
		(req.Body == nil || req.Body == http.NoBody) &&
		req.Header.Get("Range") == "" &&
		resp.Body != nil &&
		resp.StatusCode == http.StatusOK &&
		identityEncoded(resp.Header) &&
		resp.ContentLength > 0
}

// isBlobRequest reports whether a request is fetching a /v2/<name>/blobs/<digest> URL. the request
// seen here is often a CDN hop, so the match is on the request the redirects began with.
func isBlobRequest(req *http.Request) bool {
	// the last /blobs/, since a repository name may itself have a blobs component
	path := firstRequest(req).URL.Path
	i := strings.LastIndex(path, "/blobs/")
	if i < 0 {
		return false
	}
	digest := path[i+len("/blobs/"):]

	// an upload session lives under /blobs/uploads/, and a digest is algorithm:hex
	return strings.Contains(digest, ":") && !strings.Contains(digest, "/")
}

// firstRequest walks back through any redirects to the request the client began with.
func firstRequest(req *http.Request) *http.Request {
	for req.Response != nil && req.Response.Request != nil {
		req = req.Response.Request
	}

	return req
}

// resumableBody is the io.ReadCloser handed back in place of a blob's response body.
type resumableBody struct {
	t     *resumableTransport
	req   *http.Request
	total int64

	// mu guards body, cancel and closed, since closing a response body from another goroutine to
	// abort a read is legal and Close may arrive while Read is blocked or mid-resume
	mu     sync.Mutex
	body   io.ReadCloser
	closed bool

	// cancel ends the reopen in flight, or the body it produced, so Close can reach a reopen that has
	// not returned yet
	cancel context.CancelFunc

	// done is shut by Close, so a backoff in progress is abandoned
	done chan struct{}

	// the rest belong to the single reader and are not guarded
	offset int64
	// spanEnd is the offset the current segment's Content-Range said it would stop at
	spanEnd  int64
	progress int64
	stalls   int
	resumes  int
	// failed latches the error that ended the stream, so reading past it never starts over
	failed error
}

func (b *resumableBody) Read(p []byte) (int, error) {
	if b.failed != nil {
		return 0, b.failed
	}

	for {
		body, err := b.currentBody() //nolint:closecheck // borrowed: b.body is owned by b and closed by Close/closeCurrent
		if err != nil {
			return 0, err
		}

		n, readErr := body.Read(p)
		b.record(n)

		if readErr == nil {
			return n, nil
		}
		if b.isClosed() {
			return n, b.latch(http.ErrBodyReadAfterClose)
		}
		if done, final := b.finished(); done {
			// release the connection and any resume's context now, as an unwrapped body would at EOF,
			// rather than leaving them for a Close that may never come
			b.closeCurrent()

			return n, b.latch(final)
		}
		// a segment that stopped where its Content-Range said it would, as on a CDN that caps spans,
		// rather than a dropped connection. io.EOF alone does not say so, since a span framed by
		// connection close reports it on a drop too
		spanEnded := b.offset == b.spanEnd
		if resumeErr := b.resume(quoted(shortRead(readErr)), spanEnded); resumeErr != nil {
			return n, b.latch(resumeErr)
		}
		if n > 0 {
			// hand back what this read managed; the next call continues on the new connection
			return n, nil
		}
	}
}

// shortRead renames the io.EOF of a stream known to be short. the terminal error wraps its cause, and
// errors.Is(err, io.EOF) on a truncation would read as a clean end of stream: file.IterateTar and
// several go-containerregistry helpers stop there without error, turning a failed layer into a
// silently short SBOM.
func shortRead(readErr error) error {
	if errors.Is(readErr, io.EOF) {
		return io.ErrUnexpectedEOF
	}

	return readErr
}

// latch records the error that ended the stream so every subsequent Read returns it.
func (b *resumableBody) latch(err error) error {
	b.failed = err

	return err
}

// record accounts for bytes just delivered, replenishing the stall budget once enough have arrived.
func (b *resumableBody) record(n int) {
	if n <= 0 {
		return
	}

	b.offset += int64(n)
	b.progress += int64(n)

	if b.progress >= b.t.minProgress {
		b.stalls = 0
		b.progress = 0
	}
}

// finished reports whether the read is over, and with which error. a body that arrived in full is
// finished whatever error came with its last bytes, since a connection torn down right after them
// reports something other than io.EOF. a cancelled context ends the read as itself.
func (b *resumableBody) finished() (bool, error) {
	if b.offset >= b.total {
		return true, io.EOF
	}

	if ctxErr := b.req.Context().Err(); ctxErr != nil {
		return true, ctxErr
	}

	return false, nil
}

// resume closes the failed connection and reopens the request from the current offset, retrying
// until it succeeds or runs out of budget.
//
// spanEnded says the segment stopped exactly where its own Content-Range declared, as on a CDN that
// caps spans. the server answered correctly and promptly, so that is not a drop: the first attempt
// after one spends no total budget and waits no backoff. its retries are drops like any other.
func (b *resumableBody) resume(cause error, spanEnded bool) error {
	b.closeCurrent()

	// a drop is info, since it is the only sign an operator has that a slow pull is recovering rather
	// than wedged. a capped span is routine and would otherwise log one line per span. it carries no
	// error either: the stream ended where the server said it would, and cause has been renamed to
	// io.ErrUnexpectedEOF for the terminal error it may yet become, which is not what happened here
	if spanEnded {
		log.WithFields("url", forLog(b.req), "offset", b.offset).
			Debug("blob span ended, continuing from the current offset")
	} else {
		log.WithFields("url", forLog(b.req), "offset", b.offset, "error", cause).
			Info("blob stream ended early, resuming from the current offset")
	}

	for attempt := 0; ; attempt++ {
		// the first reopen after a capped span is routine rather than a recovery, so it is charged
		// neither the total budget nor the backoff
		routine := attempt == 0 && spanEnded

		if !routine {
			b.resumes++
			if b.resumes > b.t.maxResumes+int(b.offset/b.t.resumeCredit) {
				return fmt.Errorf("unable to resume %s at byte %d after %d resumes: %w",
					forLog(b.req), b.offset, b.resumes-1, cause)
			}
		}

		// every attempt spends a stall, routine or not. that is what bounds a server whose spans are
		// too small to be worth a round trip: they replenish nothing, so the budget runs out
		b.stalls++
		if b.stalls > b.t.maxStalls {
			return fmt.Errorf("unable to resume %s at byte %d after %d attempts delivered less than %d bytes between them: %w",
				forLog(b.req), b.offset, b.t.maxStalls, b.t.minProgress, cause)
		}

		// sleep is called even with no delay, so cancellation and Close are noticed on every attempt.
		// a routine reopen waits no backoff: there is nothing to back off from
		var delay time.Duration
		if b.stalls > 1 && !routine {
			delay = time.Duration(b.stalls-1) * b.t.backoff
		}

		if err := b.sleep(delay); err != nil {
			return err
		}

		body, err := b.reopen() //nolint:closecheck // ownership passes to adopt, which installs or closes it
		if err != nil {
			// a Close or a cancelled pull ends the reopen by design, and the read with it
			if b.isClosed() {
				return http.ErrBodyReadAfterClose
			}

			if ctxErr := b.req.Context().Err(); ctxErr != nil {
				return ctxErr
			}

			if errors.Is(err, errResumeRefused) {
				return fmt.Errorf("unable to resume %s at byte %d after %w: %w",
					forLog(b.req), b.offset, cause, err)
			}

			// a peer that reads the request and closes without answering gives a bare io.EOF here too
			cause = quoted(shortRead(err))
			log.WithFields("url", forLog(b.req), "offset", b.offset, "error", cause).
				Debug("unable to resume blob stream, retrying")

			continue
		}

		return b.adopt(body)
	}
}

// adopt installs a freshly opened body, unless the caller closed us while it was being opened.
func (b *resumableBody) adopt(body io.ReadCloser) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		closeQuietly(body)

		return http.ErrBodyReadAfterClose
	}

	b.body = body

	return nil
}

// sleep waits for d, or until the request's context is cancelled or the body closed. the checks come
// before the zero-delay short-circuit, so a zero backoff behaves the same way.
func (b *resumableBody) sleep(d time.Duration) error {
	ctx := b.req.Context()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.done:
		return http.ErrBodyReadAfterClose
	default:
	}

	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.done:
		return http.ErrBodyReadAfterClose
	case <-timer.C:
		return nil
	}
}

// reopen issues the blob request again for the bytes not yet delivered.
//
// the range is closed: go-containerregistry's own registry (`crane registry serve`, ko) parses it with
// Sscanf("bytes=%d-%d") and answers 416 to an open-ended "bytes=N-". Accept-Ranges is not checked
// first, since most registries honour a range without advertising it.
func (b *resumableBody) reopen() (io.ReadCloser, error) {
	ctx, cancel := context.WithCancel(b.req.Context())
	if !b.adoptCancel(cancel) {
		cancel()

		return nil, http.ErrBodyReadAfterClose
	}

	headers := time.AfterFunc(b.t.idleTimeout, cancel)
	resp, err := b.roundTrip(ctx)

	if !headers.Stop() {
		// the cancellation was ours, so it must not read as context.Canceled to the caller. a
		// response that raced the timer is already cancelled, so it is discarded
		if err == nil {
			closeQuietly(resp.Body)
		}

		return nil, fmt.Errorf("no response to the resumed range request within %s", b.t.idleTimeout)
	}

	// only an accepted body still needs its context
	if err != nil {
		cancel()

		return nil, err
	}

	body, err := b.acceptable(resp) //nolint:closecheck // ownership passes to watchIdle, whose wrapper is returned
	if err != nil {
		cancel()

		return nil, err
	}

	watched := watchIdle(body, b.t.idleTimeout)
	if resp.StatusCode == http.StatusOK {
		log.WithFields("url", forLog(b.req), "offset", b.offset).
			Info("the server ignored the range, reading the blob again from the start")

		return &skipBody{idleBody: watched, skip: b.offset}, nil
	}

	return watched, nil
}

// roundTrip sends the range request to the registry URL the blob was first requested from, rather
// than to the CDN hop that dropped, so the client can authenticate it and follow a fresh redirect.
// identity is asked for since the stream so far is wire bytes.
func (b *resumableBody) roundTrip(ctx context.Context) (*http.Response, error) {
	client, err := b.t.clientFor(ctx)
	if err != nil {
		return nil, err
	}

	// a fresh request rather than a clone: go-containerregistry's auth transports set Authorization
	// on the request in place, and a clone would carry it into the client's redirects, to hosts
	// that pull never sent it to. the reopen client adds its own for the registry host
	first := firstRequest(b.req)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, first.URL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", b.offset, b.total-1))
	req.Header.Set("Accept-Encoding", "identity")

	resp, err := client.Do(req)

	// the client names the URL in its errors, and after a redirect that is a signed CDN URL
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	return resp, err
}

// checkRedirect is go-containerregistry's checkRedirectSSRF for the reopen client: a redirect may
// change host, but not to a loopback, private, link-local or unspecified address. it is stricter in
// two ways. a host made only of numbers is refused rather than decoded, since the resolver accepts
// legacy spellings such as 127.1 that netip does not. a host that is not plain ASCII is refused, since
// the transport maps full-width digits onto an address netip never sees. that also turns away a
// registry redirecting to a genuine IDN host, which folding the name first would not, but folding
// means taking a direct dependency on x/net/idna for a case no registry has been seen to need.
//
// a redirect from https to http is followed, as go-containerregistry follows it for the pull. the
// idle clock can send a healthy pull through here, so refusing what the pull already accepted (a
// registry fronting plain-http storage, say) would fail a pull that waiting would have finished.
// credentials are not exposed by it: go-containerregistry only attaches them for the registry host,
// whose scheme its schemeTransport puts back, and the request a resume builds starts without any.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("%w: stopped after 10 redirects", errResumeRefused)
	}

	from, to := via[0].URL.Hostname(), req.URL.Hostname()
	if to == from {
		return nil
	}

	addr, err := netip.ParseAddr(to)
	if err != nil {
		if !isASCII(to) || numericHost(to) {
			return fmt.Errorf("%w: redirect to the host %q denied", errResumeRefused, clip(to))
		}

		return nil
	}

	if addr = addr.Unmap(); addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
		return fmt.Errorf("%w: redirect from %q to private host %q denied", errResumeRefused, clip(from), clip(to))
	}

	return nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

// numericHost reports whether every dot-separated part of a host is a number in some base, as in
// 127.1, 0x7f000001 or 2130706433.
func numericHost(host string) bool {
	for _, part := range strings.Split(host, ".") {
		if _, err := strconv.ParseUint(part, 0, 32); err != nil {
			return false
		}
	}

	return true
}

// adoptCancel stores the cancel belonging to the reopen now in flight, ending any it replaces, and
// reports whether the caller is still wanted, since Close may have arrived while it was being opened.
func (b *resumableBody) adoptCancel(cancel context.CancelFunc) bool {
	b.mu.Lock()
	previous := b.cancel
	b.cancel = cancel
	closed := b.closed
	b.mu.Unlock()

	if previous != nil {
		previous()
	}

	return !closed
}

// acceptable decides whether a reissued response may be spliced onto the stream, closing it if not.
func (b *resumableBody) acceptable(resp *http.Response) (io.ReadCloser, error) {
	reject := func(format string, args ...any) (io.ReadCloser, error) {
		closeQuietly(resp.Body)

		return nil, fmt.Errorf("%w: "+format, append([]any{errResumeRefused}, args...)...)
	}

	// a server with no range support answers with the whole blob. that is still usable once what was
	// already delivered is skipped (reopen does that), costing a re-download rather than the layer.
	// anything other than the whole blob, in the identity coding, cannot be lined up with the stream
	if resp.StatusCode == http.StatusOK && resp.ContentLength == b.total && identityEncoded(resp.Header) {
		b.spanEnd = b.total

		return resp.Body, nil
	}

	if resp.StatusCode != http.StatusPartialContent {
		closeQuietly(resp.Body)

		return nil, b.refusedStatus(resp)
	}

	// after the status check, so a busy server's compressed error page is not a permanent refusal
	if !identityEncoded(resp.Header) {
		return reject("the resumed segment came back encoded")
	}

	start, end, total, err := parseContentRange(resp.Header.Get("Content-Range"))
	if err != nil {
		return reject("%w", err)
	}

	if start != b.offset {
		return reject("server resumed at byte %d, expected %d", start, b.offset)
	}

	if total >= 0 && total != b.total {
		return reject("the blob is now %d bytes, it was %d when the read began", total, b.total)
	}

	// a server that ignored the Range and restarted the object declares more than the span
	if span := end - start + 1; resp.ContentLength >= 0 && resp.ContentLength != span {
		return reject("Content-Range covers %d bytes but %d were sent", span, resp.ContentLength)
	}

	if end >= b.total {
		return reject("Content-Range runs to byte %d, past the end at %d", end, b.total-1)
	}

	// a span may stop short of the end (some CDNs cap them), and Read resumes again when it runs
	// out. the body is held to the span, since one with no declared length could run past it
	b.spanEnd = end + 1

	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(resp.Body, end-start+1), resp.Body}, nil
}

// refusedStatus classifies a reopen that did not come back as 206. a busy server is worth another
// attempt, anything else is permanent, including a 401 or 403 since the client has already
// re-authorized and followed a fresh redirect to get it.
func (b *resumableBody) refusedStatus(resp *http.Response) error {
	switch code := resp.StatusCode; {
	case code == http.StatusRequestTimeout || code == http.StatusTooManyRequests ||
		code == statusClientClosedRequest || code >= http.StatusInternalServerError:
		// what go-containerregistry's retry transport treats as transient, and the rest of 5xx
		return fmt.Errorf("the server could not serve the range just now: got %d resuming at byte %d", code, b.offset)

	default:
		return fmt.Errorf("%w: expected HTTP 206 resuming at byte %d, got %d", errResumeRefused, b.offset, code)
	}
}

// identityEncoded reports whether every Content-Encoding line is empty or "identity", which some
// servers echo back from the Accept-Encoding a reopen sends.
func identityEncoded(header http.Header) bool {
	for _, encoding := range header.Values("Content-Encoding") {
		if encoding = strings.TrimSpace(encoding); encoding != "" && !strings.EqualFold(encoding, "identity") {
			return false
		}
	}

	return true
}

// currentBody returns the body to read from, or an error if the caller has closed us.
func (b *resumableBody) currentBody() (io.ReadCloser, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed || b.body == nil {
		return nil, http.ErrBodyReadAfterClose
	}

	return b.body, nil
}

func (b *resumableBody) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.closed
}

// closeCurrent drops the failed connection without marking the body closed to callers.
func (b *resumableBody) closeCurrent() {
	if body, _ := b.release(false); body != nil { //nolint:closecheck // closed by closeQuietly
		closeQuietly(body)
	}
}

func (b *resumableBody) Close() error {
	body, first := b.release(true)
	if first {
		// wakes a backoff in progress; guarded by first so a second Close cannot panic
		close(b.done)
	}

	if body == nil {
		return nil
	}

	return body.Close()
}

// release detaches the current body and ends the reopen that produced it, or one still in flight,
// which closing the body cannot reach. closing also marks b closed, and reports whether this call was
// the one that did.
func (b *resumableBody) release(closing bool) (io.ReadCloser, bool) {
	b.mu.Lock()
	first := closing && !b.closed
	b.closed = b.closed || closing
	body, cancel := b.body, b.cancel
	b.body, b.cancel = nil, nil
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	return body, first
}

// skipBody discards the first skip bytes of a whole blob sent in answer to a range. they are read
// through the idle clock in small pieces, so a server falling silent while they are skipped is a drop
// like any other rather than a read blocked for ever.
type skipBody struct {
	*idleBody
	skip int64
}

func (s *skipBody) Read(p []byte) (int, error) {
	if s.skip > 0 {
		n, err := io.CopyN(io.Discard, s.idleBody, s.skip)
		s.skip -= n
		if err != nil {
			return 0, err
		}
	}

	return s.idleBody.Read(p)
}

// idleBody closes a segment whose read waits longer than timeout for any bytes, so a connection that
// stays open but falls silent becomes a drop that can be resumed rather than a read blocked for ever.
// the clock runs only while a read is waiting on the server, never between reads, so a slow consumer
// does not look like a silent server.
type idleBody struct {
	io.ReadCloser
	timeout time.Duration
	timer   *time.Timer
}

func watchIdle(body io.ReadCloser, timeout time.Duration) *idleBody {
	b := &idleBody{ReadCloser: body, timeout: timeout}
	b.timer = time.AfterFunc(timeout, func() { closeQuietly(body) })
	b.timer.Stop()

	return b
}

func (b *idleBody) Read(p []byte) (int, error) {
	b.timer.Reset(b.timeout)
	n, err := b.ReadCloser.Read(p)
	if !b.timer.Stop() {
		// the timer fired and closed the body, so even bytes that raced it are the last this read
		// gets. the error is whatever closing made the read return, which says nothing useful
		err = fmt.Errorf("no data received within %s", b.timeout)
	}

	return n, err
}

func (b *idleBody) Close() error {
	b.timer.Stop()

	return b.ReadCloser.Close()
}

// parseContentRange returns the first and last byte and the total of a Content-Range such as
// "bytes 4096-8191/16384". an unknown total ("*") is -1.
func parseContentRange(header string) (int64, int64, int64, error) {
	malformed := func() (int64, int64, int64, error) {
		return 0, 0, 0, fmt.Errorf("malformed Content-Range %q", clip(header))
	}

	unit, spec, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(unit, "bytes") {
		return malformed()
	}

	span, size, ok := strings.Cut(strings.TrimSpace(spec), "/")
	if !ok {
		return malformed()
	}

	first, last, ok := strings.Cut(span, "-")
	if !ok {
		return malformed()
	}

	start, err := strconv.ParseInt(strings.TrimSpace(first), 10, 64)
	if err != nil {
		return malformed()
	}

	end, err := strconv.ParseInt(strings.TrimSpace(last), 10, 64)
	if err != nil || start < 0 || end < start {
		return malformed()
	}

	if size = strings.TrimSpace(size); size == "*" {
		return start, end, -1, nil
	}

	total, err := strconv.ParseInt(size, 10, 64)
	if err != nil || total <= end {
		return malformed()
	}

	return start, end, total, nil
}

// forLog names a blob by the registry URL it was requested from, never a signed CDN URL.
func forLog(req *http.Request) string {
	u := firstRequest(req).URL

	// escaped, since a foreign layer URL comes from the manifest and its path may decode to anything
	return u.Host + u.EscapedPath()
}

// clip caps text a server chose before it goes into a log line or an error.
func clip(s string) string {
	if len(s) > maxErrorText {
		return s[:maxErrorText] + "..."
	}

	return s
}

// quotedError keeps an error's identity for errors.Is, but caps its text and escapes control
// characters. a failed reopen's error can carry bytes the server chose (some Go releases put a
// malformed header line into it raw), and it is logged and handed to the caller.
type quotedError struct{ err error }

func quoted(err error) error {
	if err == nil {
		return nil
	}

	return quotedError{err: err}
}

func (e quotedError) Error() string {
	q := strconv.Quote(clip(e.err.Error()))

	return q[1 : len(q)-1]
}

func (e quotedError) Unwrap() error { return e.err }

func closeQuietly(c io.Closer) {
	if err := c.Close(); err != nil {
		log.WithFields("error", err).Trace("unable to close dropped blob stream")
	}
}
