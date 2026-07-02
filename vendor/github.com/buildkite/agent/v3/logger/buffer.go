package logger

import (
	"fmt"
	"sync"
)

// Buffer is a Logger implementation intended for testing;
// messages are stored internally.
type Buffer struct {
	mu       sync.Mutex
	Messages []string
}

// NewBuffer creates a new Buffer with Messages slice initialized.
// This makes it simpler to assert empty []string when no log messages
// have been sent; otherwise Messages would be nil.
func NewBuffer() *Buffer {
	return &Buffer{
		Messages: make([]string, 0),
	}
}

func (b *Buffer) Debug(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[debug] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Error(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[error] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Fatal(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[fatal] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Notice(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[notice] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Warn(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[warn] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Info(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[info] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) WithFields(fields ...Field) Logger {
	return b
}
func (b *Buffer) SetLevel(level Level) {}
func (b *Buffer) Level() Level {
	return 0
}
