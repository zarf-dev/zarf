package devslog

type (
	foregroundColor   []byte
	backgroundColor   []byte
	commonValuesColor []byte
)

type color struct {
	fg foregroundColor
	bg backgroundColor
}

var (
	// Foreground colors
	fgBlack   foregroundColor = []byte("\x1b[30m")
	fgRed     foregroundColor = []byte("\x1b[31m")
	fgGreen   foregroundColor = []byte("\x1b[32m")
	fgYellow  foregroundColor = []byte("\x1b[33m")
	fgBlue    foregroundColor = []byte("\x1b[34m")
	fgMagenta foregroundColor = []byte("\x1b[35m")
	fgCyan    foregroundColor = []byte("\x1b[36m")
	fgWhite   foregroundColor = []byte("\x1b[37m")

	// Background colors
	bgBlack   backgroundColor = []byte("\x1b[40m")
	bgRed     backgroundColor = []byte("\x1b[41m")
	bgGreen   backgroundColor = []byte("\x1b[42m")
	bgYellow  backgroundColor = []byte("\x1b[43m")
	bgBlue    backgroundColor = []byte("\x1b[44m")
	bgMagenta backgroundColor = []byte("\x1b[45m")
	bgCyan    backgroundColor = []byte("\x1b[46m")
	bgWhite   backgroundColor = []byte("\x1b[47m")

	// Common consts
	resetColor     commonValuesColor = []byte("\x1b[0m")
	faintColor     commonValuesColor = []byte("\x1b[2m")
	underlineColor commonValuesColor = []byte("\x1b[4m")
)

type Color uint

const (
	UnknownColor Color = iota
	Black
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

var colors = []color{
	{},
	{fgBlack, bgBlack},
	{fgRed, bgRed},
	{fgGreen, bgGreen},
	{fgYellow, bgYellow},
	{fgBlue, bgBlue},
	{fgMagenta, bgMagenta},
	{fgCyan, bgCyan},
	{fgWhite, bgWhite},
}

func (h *developHandler) getColor(c Color) color {
	if int(c) < len(colors) {
		return colors[c]
	}

	return colors[White]
}

// Color string foreground
func (h *developHandler) colorString(b []byte, fgColor foregroundColor) []byte {
	if h.opts.NoColor {
		return b
	}

	b = append(fgColor, b...)
	b = append(b, resetColor...)
	return b
}

// Color string fainted
func (h *developHandler) colorStringFainted(b []byte, fgColor foregroundColor) []byte {
	if h.opts.NoColor {
		return b
	}

	b = append(fgColor, b...)
	b = append(faintColor, b...)
	b = append(b, resetColor...)
	return b
}

// Color string background
func (h *developHandler) colorStringBackgorund(b []byte, fgColor foregroundColor, bgColor backgroundColor) []byte {
	if h.opts.NoColor {
		return b
	}

	b = append(fgColor, b...)
	b = append(bgColor, b...)
	b = append(b, resetColor...)
	return b
}

// Underline text
func (h *developHandler) underlineText(b []byte) []byte {
	if h.opts.NoColor {
		return b
	}

	b = append(underlineColor, b...)
	b = append(b, resetColor...)
	return b
}

// Fainted text
func (h *developHandler) faintedText(b []byte) []byte {
	if h.opts.NoColor {
		return b
	}

	b = append(faintColor, b...)
	b = append(b, resetColor...)
	return b
}
