// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zarf-dev/zarf/src/pkg/message"
)

// PtermHandler is a slog handler that prints using Pterm.
type PtermHandler struct {
	attrs []slog.Attr
	group string
}

// NewPtermHandler returns a new instance of PtermHandler.
func NewPtermHandler() *PtermHandler {
	return &PtermHandler{}
}

//nolint:revive // ignore
func (h *PtermHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

//nolint:revive // ignore
func (h *PtermHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := []string{}
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a.String())
		return true
	})
	out := fmt.Sprintf("%s %s", r.Message, strings.Join(attrs, " "))
	switch r.Level {
	case slog.LevelDebug:
		message.Debug(out)
	case slog.LevelInfo:
		message.Info(out)
	case slog.LevelWarn:
		message.Warn(out)
	case slog.LevelError:
		message.Warn(out)
	}
	return nil
}

//nolint:revive // ignore
func (h *PtermHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &PtermHandler{
		attrs: append(h.attrs, attrs...),
		group: h.group,
	}
}

//nolint:revive // ignore
func (h *PtermHandler) WithGroup(name string) slog.Handler {
	return &PtermHandler{
		attrs: h.attrs,
		group: name,
	}
}
