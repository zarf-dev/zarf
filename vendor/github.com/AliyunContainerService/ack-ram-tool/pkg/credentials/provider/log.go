package provider

import (
	"log"
	"os"
	"strings"
)

var debugMode bool

type Logger interface {
	Info(msg string)
	Debug(msg string)
	Error(err error, msg string)
}

var defaultLog Logger = defaultLogger{}

func init() {
	debugEnv := strings.Split(strings.ToLower(os.Getenv("DEBUG")), ",")
	for _, item := range debugEnv {
		if item == "sdk" || item == "tea" || item == "credentials-provider" {
			debugMode = true
			break
		}
	}
}

type defaultLogger struct {
}

func (d defaultLogger) Info(msg string) {
	log.Print(msg)
}

func (d defaultLogger) Debug(msg string) {
	if debugMode {
		log.Print(msg)
	}
}

func (d defaultLogger) Error(err error, msg string) {
	log.Print(msg)
}
