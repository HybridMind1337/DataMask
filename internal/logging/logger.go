package logging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/HybridMind1337/DataMask/internal/security"
)

// SanitizingHandler wraps an slog.Handler to scrub sensitive data patterns from messages and attributes.
type SanitizingHandler struct {
	inner slog.Handler
}

// NewSanitizingHandler creates a wrapping handler that scrubs text.
func NewSanitizingHandler(inner slog.Handler) *SanitizingHandler {
	return &SanitizingHandler{inner: inner}
}

func (h *SanitizingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *SanitizingHandler) Handle(ctx context.Context, r slog.Record) error {
	scrubbedMsg := security.ScrubMessage(r.Message)
	newRecord := slog.NewRecord(r.Time, r.Level, scrubbedMsg, r.PC)
	
	r.Attrs(func(a slog.Attr) bool {
		if a.Value.Kind() == slog.KindString {
			a.Value = slog.StringValue(security.ScrubMessage(a.Value.String()))
		}
		newRecord.AddAttrs(a)
		return true
	})
	
	return h.inner.Handle(ctx, newRecord)
}

func (h *SanitizingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	sanitized := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		if a.Value.Kind() == slog.KindString {
			sanitized[i] = slog.Attr{
				Key:   a.Key,
				Value: slog.StringValue(security.ScrubMessage(a.Value.String())),
			}
		} else {
			sanitized[i] = a
		}
	}
	return &SanitizingHandler{inner: h.inner.WithAttrs(sanitized)}
}

func (h *SanitizingHandler) WithGroup(name string) slog.Handler {
	return &SanitizingHandler{inner: h.inner.WithGroup(name)}
}

// NewLogger creates a production structured logger that prevents PII leaks.
func NewLogger(out io.Writer, verbose bool) *slog.Logger {
	if out == nil {
		out = os.Stderr
	}
	lvl := slog.LevelInfo
	if verbose {
		lvl = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{
		Level: lvl,
	}
	textHandler := slog.NewTextHandler(out, opts)
	return slog.New(NewSanitizingHandler(textHandler))
}
