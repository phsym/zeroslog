package zeroslog

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type DummyHandler struct{}

func (*DummyHandler) Enabled(context.Context, slog.Level) bool   { return true }
func (*DummyHandler) Handle(context.Context, slog.Record) error  { return nil }
func (h *DummyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *DummyHandler) WithGroup(name string) slog.Handler       { return h }

var (
	handlers = map[string]slog.Handler{
		"std-text": slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
		"std-json": slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
		"zerolog":  NewJsonHandler(io.Discard, &HandlerOptions{Level: slog.LevelDebug}),
		"dummy":    &DummyHandler{},
	}

	loggers = func() map[string]*slog.Logger {
		m := make(map[string]*slog.Logger, len(handlers))
		for name, hdl := range handlers {
			m[name] = slog.New(hdl)
		}
		return m
	}()
)

func BenchmarkZerolog_Raw(b *testing.B) {
	l := zerolog.New(io.Discard).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	l = l.With().Str("foo", "bar").Logger()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().Str("bar", "baz").Msg("hello")
	}
}

func BenchmarkHandlers(b *testing.B) {
	ctx := context.Background()
	for name, h := range handlers {
		b.Run(name, func(b *testing.B) {
			h = h.WithAttrs([]slog.Attr{slog.String("foo", "bar")})
			rec := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
			rec.AddAttrs(slog.String("bar", "baz"))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.Handle(ctx, rec)
			}
		})
	}
}

func BenchmarkLoggers(b *testing.B) {
	ctx := context.Background()
	for name, l := range loggers {
		b.Run(name, func(b *testing.B) {
			l = l.With("foo", "bar")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				l.LogAttrs(ctx, slog.LevelInfo, "hello", slog.String("bar", "baz"))
			}
		})
	}

}
