package console

import (
	"fmt"
	"log/slog"
)

type ANSIMod string

var ResetMod = ToANSICode(Reset)

const (
	Reset = iota
	Bold
	Faint
	Italic
	Underline
	CrossedOut = 9
)

const (
	Black = iota + 30
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	Gray
)

const (
	BrightBlack = iota + 90
	BrightRed
	BrightGreen
	BrightYellow
	BrightBlue
	BrightMagenta
	BrightCyan
	White
)

func (c ANSIMod) String() string {
	return string(c)
}

func ToANSICode(modes ...int) ANSIMod {
	if len(modes) == 0 {
		return ""
	}

	var s string
	for i, m := range modes {
		if i > 0 {
			s += ";"
		}
		s += fmt.Sprintf("%d", m)
	}
	return ANSIMod("\x1b[" + s + "m")
}

type Theme interface {
	Name() string
	Timestamp() ANSIMod
	Source() ANSIMod

	Message() ANSIMod
	MessageDebug() ANSIMod
	AttrKey() ANSIMod
	AttrValue() ANSIMod
	AttrValueError() ANSIMod
	LevelError() ANSIMod
	LevelWarn() ANSIMod
	LevelInfo() ANSIMod
	LevelDebug() ANSIMod
	Level(level slog.Level) ANSIMod
}

type ThemeDef struct {
	name           string
	timestamp      ANSIMod
	source         ANSIMod
	message        ANSIMod
	messageDebug   ANSIMod
	attrKey        ANSIMod
	attrValue      ANSIMod
	attrValueError ANSIMod
	levelError     ANSIMod
	levelWarn      ANSIMod
	levelInfo      ANSIMod
	levelDebug     ANSIMod
}

func (t ThemeDef) Name() string            { return t.name }
func (t ThemeDef) Timestamp() ANSIMod      { return t.timestamp }
func (t ThemeDef) Source() ANSIMod         { return t.source }
func (t ThemeDef) Message() ANSIMod        { return t.message }
func (t ThemeDef) MessageDebug() ANSIMod   { return t.messageDebug }
func (t ThemeDef) AttrKey() ANSIMod        { return t.attrKey }
func (t ThemeDef) AttrValue() ANSIMod      { return t.attrValue }
func (t ThemeDef) AttrValueError() ANSIMod { return t.attrValueError }
func (t ThemeDef) LevelError() ANSIMod     { return t.levelError }
func (t ThemeDef) LevelWarn() ANSIMod      { return t.levelWarn }
func (t ThemeDef) LevelInfo() ANSIMod      { return t.levelInfo }
func (t ThemeDef) LevelDebug() ANSIMod     { return t.levelDebug }
func (t ThemeDef) Level(level slog.Level) ANSIMod {
	switch {
	case level >= slog.LevelError:
		return t.LevelError()
	case level >= slog.LevelWarn:
		return t.LevelWarn()
	case level >= slog.LevelInfo:
		return t.LevelInfo()
	default:
		return t.LevelDebug()
	}
}

func NewDefaultTheme() Theme {
	return ThemeDef{
		name:           "Default",
		timestamp:      ToANSICode(BrightBlack),
		source:         ToANSICode(Bold, BrightBlack),
		message:        ToANSICode(Bold),
		messageDebug:   ToANSICode(),
		attrKey:        ToANSICode(Cyan),
		attrValue:      ToANSICode(),
		attrValueError: ToANSICode(Bold, Red),
		levelError:     ToANSICode(Red),
		levelWarn:      ToANSICode(Yellow),
		levelInfo:      ToANSICode(Green),
		levelDebug:     ToANSICode(),
	}
}

func NewBrightTheme() Theme {
	return ThemeDef{
		name:           "Bright",
		timestamp:      ToANSICode(Gray),
		source:         ToANSICode(Bold, Gray),
		message:        ToANSICode(Bold, White),
		messageDebug:   ToANSICode(),
		attrKey:        ToANSICode(BrightCyan),
		attrValue:      ToANSICode(),
		attrValueError: ToANSICode(Bold, BrightRed),
		levelError:     ToANSICode(BrightRed),
		levelWarn:      ToANSICode(BrightYellow),
		levelInfo:      ToANSICode(BrightGreen),
		levelDebug:     ToANSICode(),
	}
}
