package archives

import (
	"bytes"
	"context"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
)

func init() {
	RegisterFormat(Brotli{})
}

// Brotli facilitates brotli compression.
type Brotli struct {
	Quality int
}

func (Brotli) Extension() string { return ".br" }
func (Brotli) MediaType() string { return "application/x-br" }

func (br Brotli) Match(ctx context.Context, filename string, stream io.Reader) (MatchResult, error) {
	var mr MatchResult

	// match filename
	if strings.Contains(strings.ToLower(filename), br.Extension()) {
		mr.ByName = true
	}

	if stream != nil {
		mr.ByStream = br.isValidBrotliStream(ctx, stream)
	}

	return mr, nil
}

func (br Brotli) isValidBrotliStream(ctx context.Context, stream io.Reader) bool {
	// brotli does not have well-defined file headers or a magic number;
	// the best way to match the stream is to try decoding a small amount
	// and see if it succeeds without errors

	readTarget := 1024

	limitedStream, err := readAtMost(stream, readTarget)
	if err != nil {
		return false
	}
	input := &bytes.Buffer{}
	r := brotli.NewReader(io.TeeReader(bytes.NewReader(limitedStream), input))

	// Read more data to get a better compression ratio estimate
	output := &bytes.Buffer{}
	buf := make([]byte, len(limitedStream))

	totalRead := 0
	// Try to read up to 1KB of decompressed data
	for totalRead < readTarget {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return false
		}
		if n == 0 {
			break
		}
		output.Write(buf[:n])
		totalRead += n
		if err == io.EOF {
			break
		}
	}

	inputBytes := input.Bytes()
	outputBytes := output.Bytes()

	// the brotli detection often has false positives; while it is bad if we think it's brotli and it's
	// actually not compressed, it's truly tragic when we think it's brotli but it's actually another
	// format that we would/do properly detect -- avoid stepping on other formats
	for _, format := range formats {
		if format.Extension() == br.Extension() {
			continue
		}
		// this is not super efficient; we could probably handle this brotli special case a little better
		result, _ := format.Match(ctx, "", bytes.NewReader(inputBytes))
		if result.Matched() {
			return false
		}
	}

	expansionRatio := float64(totalRead) / float64(len(inputBytes))
	if expansionRatio > 1.0 {
		// Looks like actual decompression happened - this is good
		return true
	}

	// If the decompressed output is ASCII or UTF-8 characters
	// it's more likely to be real compressed data(?)
	if isASCII(outputBytes) || utf8.Valid(outputBytes) {
		return true
	}

	// A final special (terrible) check for valid brotli streams if we have made it this far
	// Brotli compressed data typically starts with specific bit patterns
	// Check if this looks like a valid brotli stream header
	// Note this approach has shortcomings, see: https://stackoverflow.com/a/39032023
	if len(inputBytes) >= 4 {
		firstByte := inputBytes[0]

		// From all tests in the test suite, the first byte only ever consists of:
		// - 0x1b (27): 5930 occurrences (60.93%)
		// - 0x0b (11): 3725 occurrences (38.28%)
		// - 0x8b (139): 77 occurrences (0.79%)
		if firstByte == 0x1b || firstByte == 0x0b || firstByte == 0x8b {
			return true
		}
	}

	// At this point:
	// - Input data is not ASCII
	// - Decompressed output is not ASCII
	// - Decompression "worked" but decompressed data is not much larger than the input data (can legitimately happen with brotli quality=0 and small inputs)
	// This is suggestive that it is not brotli compressed data.
	// BUT BEWARE: The current test suite does not actually reach this point.
	return false
}

// isASCII checks if the given byte slice contains only ASCII printable characters and common whitespace.
// It allows:
// - Tab (9)
// - Newline (10)
// - Vertical tab (11)
// - Form feed (12)
// - Carriage return (13)
// - Space (32) through tilde (126) - all printable ASCII characters
// It excludes all other control characters and non-ASCII bytes.
func isASCII(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, b := range data {
		if !isASCIIByte(b) {
			return false
		}
	}
	return true
}

func isASCIIByte(b byte) bool {
	// Allow tab, newline, vertical tab, form feed, carriage return
	if b >= 9 && b <= 13 {
		return true
	}
	// Allow space through tilde (printable ASCII)
	if b >= 32 && b <= 126 {
		return true
	}
	return false
}

func (br Brotli) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return brotli.NewWriterLevel(w, br.Quality), nil
}

func (Brotli) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(brotli.NewReader(r)), nil
}
