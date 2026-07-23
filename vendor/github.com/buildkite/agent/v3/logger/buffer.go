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

func (b *Buffer) Debugf(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[debug] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Errorf(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[error] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Fatalf(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[fatal] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Noticef(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[notice] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Warnf(format string, v ...any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Messages = append(b.Messages, "[warn] "+fmt.Sprintf(format, v...))
}

func (b *Buffer) Infof(format string, v ...any) {
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
