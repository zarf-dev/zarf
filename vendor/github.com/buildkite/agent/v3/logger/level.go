package logger

import (
	"fmt"
	"strings"
)

type Level int

const (
	DEBUG Level = iota
	NOTICE
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = []string{
	"DEBUG",
	"NOTICE",
	"INFO",
	"WARN",
	"ERROR",
	"FATAL",
}

func LevelFromString(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return DEBUG, nil
	case "notice":
		return NOTICE, nil
	case "info":
		return INFO, nil
	case "warn", "warning":
		return WARN, nil
	case "error":
		return ERROR, nil
	case "fatal":
		return FATAL, nil
	default:
		return -1, fmt.Errorf("invalid log level: %s. Valid levels are: %v", s, levelNames)
	}
}

// String returns the string representation of a logging level.
func (p Level) String() string {
	return levelNames[p]
}
