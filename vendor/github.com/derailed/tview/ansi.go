package tview

import (
	"bytes"
	"io"
	"math"
	"strconv"
	"strings"
)

// The states of the ANSI escape code parser.
const (
	ansiText = iota
	ansiEscape
	ansiSubstring
	ansiControlSequence
)

var colors = []string{
	"black",
	"maroon",
	"green",
	"olive",
	"navy",
	"purple",
	"teal",
	"silver",
	"gray",
	"red",
	"lime",
	"yellow",
	"blue",
	"fuchsia",
	"aqua",
	"white",
}

// ansi is a io.Writer which translates ANSI escape codes into tview color
// tags.
type ansi struct {
	io.Writer

	// Reusable buffers.
	buffer       *bytes.Buffer // The entire output text of one Write().
	csiParameter []rune        // Partial CSI strings.
	attributes   string        // The buffer's current text attributes (a tview attribute string).

	// The current state of the parser. One of the ansi constants.
	state int
	// foreground, background reset colors
	fg, bg string
}

// ANSIWriter returns an io.Writer which translates any ANSI escape codes
// written to it into tview color tags. Other escape codes don't have an effect
// and are simply removed. The translated text is written to the provided
// writer.
func ANSIWriter(writer io.Writer, fg, bg string) io.Writer {
	return &ansi{
		Writer: writer,
		state:  ansiText,
		fg:     fg,
		bg:     bg,
	}
}

// Write parses the given text as a string of runes, translates ANSI escape
// codes to color tags and writes them to the output writer.
func (a *ansi) Write(bb []byte) (int, error) {
	if a.buffer == nil {
		a.buffer = bytes.NewBuffer(make([]byte, 0, len(bb)))
	}
	defer a.buffer.Reset()

	num := func(rr []rune) int {
		var n int
		p := len(rr) - 1
		for _, r := range rr {
			n += int(r-'0') * int(math.Pow(10, float64(p)))
			p -= 1
		}
		return n
	}

	for _, r := range string(bb) {
		switch a.state {
		// We just entered an escape sequence.
		case ansiEscape:
			switch r {
			case '[': // Control Sequence Introducer.
				a.csiParameter = a.csiParameter[:0]
				a.state = ansiControlSequence
			case 'c': // Reset.
				a.buffer.WriteString("[-::-]")
				a.state = ansiText
			case 'P', ']', 'X', '^', '_': // Substrings and commands.
				a.state = ansiSubstring
			default: // Ignore.
				a.state = ansiText
			}

		// CSI Sequences.
		case ansiControlSequence:
			switch {
			case r >= 0x30 && r <= 0x3f: // Parameter bytes.
				a.csiParameter = append(a.csiParameter, r)
			case r >= 0x40 && r <= 0x7e: // Final byte.
				switch r {
				case 'E': // Next line.
					count, _ := strconv.Atoi(string(a.csiParameter))
					if count == 0 {
						count = 1
					}
					for i := 0; i < int(count); i++ {
						a.buffer.WriteByte('\n')
					}
				case 'm': // Select Graphic Rendition.
					var background, foreground string
					fields, bb := make([]int, 0, 10), make([]rune, 0, 10)
					for _, r := range a.csiParameter {
						if r == ';' {
							fields = append(fields, num(bb))
							bb = bb[:0]
						} else {
							bb = append(bb, r)
						}
					}
					if len(bb) > 0 {
						fields = append(fields, num(bb))
					}
					if len(a.csiParameter) == 0 || len(fields) == 1 && fields[0] == 0 {
						// Reset.
						a.attributes = ""
						if _, err := a.buffer.WriteString("[" + a.fg + ":" + a.bg + ":-]"); err != nil {
							return 0, err
						}
					}
					for index, field := range fields {
						switch field {
						case 1:
							if !strings.ContainsRune(a.attributes, 'b') {
								a.attributes += "b"
							}
						case 2:
							if !strings.ContainsRune(a.attributes, 'd') {
								a.attributes += "d"
							}
						case 4:
							if !strings.ContainsRune(a.attributes, 'u') {
								a.attributes += "u"
							}
						case 5:
							if !strings.ContainsRune(a.attributes, 'l') {
								a.attributes += ""
							}
						case 22:
							if i := strings.IndexRune(a.attributes, 'b'); i >= 0 {
								a.attributes = strings.Replace(a.attributes, "b", "", 1)
							}
							if i := strings.IndexRune(a.attributes, 'd'); i >= 0 {
								a.attributes = strings.Replace(a.attributes, "d", "", 1)
							}
						case 24:
							if i := strings.IndexRune(a.attributes, 'u'); i >= 0 {
								a.attributes = strings.Replace(a.attributes, "u", "", 1)
							}
						case 25:
							if i := strings.IndexRune(a.attributes, 'l'); i >= 0 {
								a.attributes = strings.Replace(a.attributes, "l", "", 1)
							}
						case 30, 31, 32, 33, 34, 35, 36, 37:
							n := field - 30
							if n < 0 || n > len(colors) {
								n = 0
							}
							foreground = colors[n]
						case 39:
							foreground = foreground + "-"
						case 40, 41, 42, 43, 44, 45, 46, 47:
							n := field - 40
							if n < 0 || n > len(colors) {
								n = 0
							}
							background = colors[n]
						case 49:
							background = background + "-"
						case 90, 91, 92, 93, 94, 95, 96, 97:
							n := field - 82
							if n < 0 || n > len(colors) {
								n = 0
							}
							foreground = colors[n]
						case 100, 101, 102, 103, 104, 105, 106, 107:
							n := field - 92
							if n < 0 || n > len(colors) {
								n = 0
							}
							background = colors[n]
						case 38, 48:
							if index+1 >= len(fields) {
								continue
							}
							var color string
							if fields[index+1] == 5 && index+2 < len(fields) { // 8-bit colors.
								colorNumber := fields[index+2]
								if colorNumber <= 15 {
									color = colors[colorNumber]
								} else if colorNumber <= 231 {
									red := (colorNumber - 16) / 36
									green := ((colorNumber - 16) / 6) % 6
									blue := (colorNumber - 16) % 6
									r := strconv.FormatInt(int64(255*red/5), 16)
									g := strconv.FormatInt(int64(255*green/5), 16)
									b := strconv.FormatInt(int64(255*blue/5), 16)
									if len(r) == 1 {
										r = "0" + r
									}
									if len(g) == 1 {
										g = "0" + g
									}
									if len(b) == 1 {
										b = "0" + b
									}
									color = "#" + r + g + b
								} else if colorNumber <= 255 {
									grey := 255 * (colorNumber - 232) / 23
									g := strconv.FormatInt(int64(grey), 16)
									if len(g) == 1 {
										g = "0" + g
									}
									color = "#" + g + g + g
								}
							} else if fields[index+1] == 2 && index+4 < len(fields) { // 24-bit colors.
								red := fields[index+2]
								green := fields[index+3]
								blue := fields[index+4]
								r := strconv.FormatInt(int64(red), 16)
								g := strconv.FormatInt(int64(green), 16)
								b := strconv.FormatInt(int64(blue), 16)
								if len(r) == 1 {
									r = "0" + r
								}
								if len(g) == 1 {
									g = "0" + g
								}
								if len(b) == 1 {
									b = "0" + b
								}
								color = "#" + r + g + b
							}

							if len(color) > 0 {
								if field == 38 {
									foreground = color
								} else {
									background = color
								}
							}
						}
					}
					if len(foreground) > 0 || len(background) > 0 || len(a.attributes) > 0 {
						a.buffer.WriteByte('[')
						a.buffer.WriteString(foreground)
						a.buffer.WriteByte(':')
						a.buffer.WriteString(background)
						a.buffer.WriteByte(':')
						a.buffer.WriteString(a.attributes)
						a.buffer.WriteByte(']')
					}
				}
				a.state = ansiText
			default: // Undefined byte.
				a.state = ansiText // Abort CSI.
			}

			// We just entered a substring/command sequence.
		case ansiSubstring:
			if r == 27 { // Most likely the end of the substring.
				a.state = ansiEscape
			} // Ignore all other characters.

			// "ansiText" and all others.
		default:
			if r == 27 {
				// This is the start of an escape sequence.
				a.state = ansiEscape
			} else {
				// Just a regular rune. Send to buffer.
				if _, err := a.buffer.WriteRune(r); err != nil {
					return 0, err
				}
			}
		}
	}

	// Write buffer to target writer.
	n, err := a.buffer.WriteTo(a.Writer)
	if err != nil {
		return int(n), err
	}
	return len(bb), nil
}

// TranslateANSI replaces ANSI escape sequences found in the provided string
// with tview's color tags and returns the resulting string.
func TranslateANSI(text []byte) []byte {
	var buffer bytes.Buffer
	writer := ANSIWriter(&buffer, "white", "black")
	writer.Write(text)
	return buffer.Bytes()
}
