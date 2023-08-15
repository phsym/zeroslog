//go:build ignorepc

package zeroslog

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"testing"
	"time"

	_ "unsafe"
)

//go:linkname IgnorePC log/slog/internal.IgnorePC
var IgnorePC bool

func BenchmarkDummy(b *testing.B) {
	ctx := context.Background()
	l := slog.New(&DummyHandler{})
	l = l.With("foo", "bar")
	b.ResetTimer()
	f := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l.LogAttrs(ctx, slog.LevelInfo, "hello", slog.String("bar", "baz"))
		}
	}
	b.Run("with-pc", f)
	b.Run("no-pc", func(b *testing.B) {
		IgnorePC = true
		defer func() {
			IgnorePC = false
		}()
		f(b)
	})
}

func BenchmarkSlogZerolog(b *testing.B) {
	ctx := context.Background()
	l := slog.New(NewJsonHandler(io.Discard, &HandlerOptions{Level: slog.LevelDebug}))
	l = l.With("foo", "bar")
	b.ResetTimer()
	f := func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l.LogAttrs(ctx, slog.LevelInfo, "hello", slog.String("bar", "baz"))
		}
	}

	b.Run("with-pc", f)
	b.Run("no-pc", func(b *testing.B) {
		IgnorePC = true
		defer func() {
			IgnorePC = false
		}()
		f(b)
	})
}

func BenchmarkSlogZerolog_HandlerWithRec(b *testing.B) {
	ctx := context.Background()
	h := slog.Handler(NewJsonHandler(io.Discard, &HandlerOptions{Level: slog.LevelDebug}))
	h = h.WithAttrs([]slog.Attr{slog.String("foo", "bar")})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
		rec.AddAttrs(slog.String("bar", "baz"))
		h.Handle(ctx, rec)
	}
}

func BenchmarkSlogZerolog_HandlerWithRecAndCaller(b *testing.B) {
	ctx := context.Background()
	h := slog.Handler(NewJsonHandler(io.Discard, &HandlerOptions{Level: slog.LevelDebug}))
	h = h.WithAttrs([]slog.Attr{slog.String("foo", "bar")})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pcx := [1]uintptr{0}
		runtime.Callers(3, pcx[:])
		rec := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", pcx[0])
		rec.AddAttrs(slog.String("bar", "baz"))
		h.Handle(ctx, rec)
	}
}

var (
	rec  slog.Record
	attr slog.Attr
	t    time.Time
)

func BenchmarkSlog(b *testing.B) {
	b.Run("new-rec", func(b *testing.B) {
		now := time.Now()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rec = slog.NewRecord(now, slog.LevelInfo, "yo", 0)
		}
	})
	b.Run("new-rec-time", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rec = slog.NewRecord(time.Now(), slog.LevelInfo, "yo", 0)
		}
	})
	b.Run("time.now", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			now = time.Now()
		}
	})
	b.Run("new-attr", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			attr = slog.String("foo", "bar")
		}
	})
	b.Run("new-add-attr", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rec := slog.NewRecord(time.Now(), slog.LevelInfo, "yo", 0)
			attr := slog.String("foo", "bar")
			rec.AddAttrs(attr)
		}
	})
	b.Run("runtime-caller.1", func(b *testing.B) {
		pc := [1]uintptr{}
		for i := 0; i < b.N; i++ {
			runtime.Callers(1, pc[:])
		}
	})
	b.Run("runtime-caller.3", func(b *testing.B) {
		pc := [1]uintptr{}
		for i := 0; i < b.N; i++ {
			runtime.Callers(3, pc[:])
		}
	})
}
