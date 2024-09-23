package zeroslog

import (
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// HandlerOptions are options for a ZerologHandler.
// A zero HandlerOptions consists entirely of default values.
type HandlerOptions struct {
	// AddSource causes the handler to compute the source code position
	// of the log statement and add a SourceKey attribute to the output.
	AddSource bool

	// Level reports the minimum record level that will be logged.
	// The handler discards records with lower levels.
	// If Level is nil, the handler assumes the level set in the logger.
	// The handler calls Level.Level if it's not nil for each record processed;
	// to adjust the minimum level dynamically, use a LevelVar.
	Level slog.Leveler
}

// zerologHandler is an internal interface used to expose additional methods
// between handlers.
type zerologHandler interface {
	slog.Handler
	// handleGroup handles records comming from the child group.
	handleGroup(group string, rec *slog.Record, e *zerolog.Event)
}

// Handler is an slog.Handler implementation that uses zerolog to process slog.Record.
type Handler struct {
	opts   *HandlerOptions
	logger zerolog.Logger
}

var _ zerologHandler = (*Handler)(nil)

// NewHandler creates a *ZerologHandler implementing slog.Handler.
// It wraps a zerolog.Logger to which log records will be sent.
//
// Unlesse opts.Level is not nil, the logger level is used to filter out records, otherwise
// opts.Level is used.
//
// The provided logger instance must be configured to not send timestamps or caller information.
//
// If opts is nil, it assumes default options values.
func NewHandler(logger zerolog.Logger, opts *HandlerOptions) *Handler {
	if opts == nil {
		opts = new(HandlerOptions)
	}
	opt := *opts // Copy
	return &Handler{
		opts:   &opt,
		logger: logger,
	}
}

// NewJsonHandler is a shortcut to calling
//
//	NewHandler(zerolog.New(out).Level(zerolog.InfoLevel), opts)
func NewJsonHandler(out io.Writer, opts *HandlerOptions) *Handler {
	return NewHandler(zerolog.New(out).Level(zerolog.InfoLevel), opts)
}

// NewConsoleHandler creates a new zerolog handler, wrapping out into a zerolog.ConsoleWriter.
// It's a shortcut to calling
//
//	NewHandler(zerolog.New(&zerolog.ConsoleWriter{Out: out, TimeFormat: time.DateTime}).Level(zerolog.InfoLevel), opts)
func NewConsoleHandler(out io.Writer, opts *HandlerOptions) *Handler {
	return NewJsonHandler(&zerolog.ConsoleWriter{Out: out, TimeFormat: time.DateTime}, opts)
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(_ context.Context, lvl slog.Level) bool {
	if h.opts.Level != nil {
		return lvl >= h.opts.Level.Level()
	}
	return zerologLevel(lvl) >= h.logger.GetLevel()
}

// startLog creates a new logging event at the given level.
func (h *Handler) startLog(lvl slog.Level) *zerolog.Event {
	logger := h.logger
	if h.opts.Level != nil {
		logger = h.logger.Level(zerologLevel(h.opts.Level.Level()))
	}
	return logger.WithLevel(zerologLevel(lvl))
}

// endLog finalize the log event by appending record source, timestamp and message before sending it.
func (h *Handler) endLog(rec *slog.Record, evt *zerolog.Event) {
	if h.opts.AddSource && rec.PC > 0 {
		frame, _ := runtime.CallersFrames([]uintptr{rec.PC}).Next()
		evt.Str(zerolog.CallerFieldName, fmt.Sprintf("%s:%d", frame.File, frame.Line))
	}

	if !rec.Time.IsZero() {
		evt.Time(zerolog.TimestampFieldName, rec.Time)
	}
	evt.Msg(rec.Message)
}

// handleGroup handles records comming from a child group.
func (h *Handler) handleGroup(group string, rec *slog.Record, dict *zerolog.Event) {
	evt := h.startLog(rec.Level)
	evt.Dict(group, dict)
	h.endLog(rec, evt)
}

// Handle implements slog.Handler.
func (h *Handler) Handle(_ context.Context, rec slog.Record) error {
	evt := h.startLog(rec.Level)
	rec.Attrs(func(a slog.Attr) bool {
		mapAttr(evt, a)
		return true
	})
	h.endLog(&rec, evt)
	return nil
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		opts:   h.opts,
		logger: mapAttrs(h.logger.With(), attrs...).Logger(),
	}
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &groupHandler{
		parent: h,
		ctx:    h.logger.With().Reset(),
		name:   strings.TrimSpace(name),
	}
}

// groupHandler handles groups and subgroups.
type groupHandler struct {
	parent zerologHandler
	ctx    zerolog.Context
	name   string
}

var _ zerologHandler = (*groupHandler)(nil)

// Enabled implements slog.Handler.
func (h *groupHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.parent.Enabled(ctx, lvl)
}

// handleGroup handles records comming from a child group.
func (h *groupHandler) handleGroup(group string, rec *slog.Record, dict *zerolog.Event) {
	l := h.ctx.Logger()
	evt := l.Log()
	evt.Dict(group, dict)
	h.parent.handleGroup(h.name, rec, evt)
}

// Handle implements slog.Handler.
func (h *groupHandler) Handle(ctx context.Context, rec slog.Record) error {
	l := h.ctx.Logger()
	evt := l.Log()
	rec.Attrs(func(a slog.Attr) bool {
		mapAttr(evt, a)
		return true
	})
	h.parent.handleGroup(h.name, &rec, evt)
	return nil
}

// WithAttrs implements slog.Handler.
func (h *groupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &groupHandler{
		parent: h.parent,
		ctx:    mapAttrs(h.ctx.Logger().With().Reset(), attrs...),
		name:   h.name,
	}
}

// WithGroup implements slog.Handler.
func (h *groupHandler) WithGroup(name string) slog.Handler {
	return &groupHandler{
		parent: h,
		ctx:    h.ctx.Logger().With().Reset(),
		name:   name,
	}
}

// zlogWriter is an interface with methods common between
// zerolog.Context and *zerolog.Event. This interface is
// implemented by both zerolog.Context and *zerolog.Event.
type zlogWriter[E any] interface {
	Bool(string, bool) E
	Dur(string, time.Duration) E
	Float64(string, float64) E
	Int64(string, int64) E
	Str(string, string) E
	Time(string, time.Time) E
	Uint64(string, uint64) E
	Dict(string, *zerolog.Event) E
	Interface(string, any) E
	AnErr(string, error) E
	Stringer(string, fmt.Stringer) E
	IPAddr(string, net.IP) E
	IPPrefix(string, net.IPNet) E
	MACAddr(string, net.HardwareAddr) E
	RawJSON(string, []byte) E
}

var (
	_ zlogWriter[*zerolog.Event]  = (*zerolog.Event)(nil)
	_ zlogWriter[zerolog.Context] = zerolog.Context{}
)

// mapAttrs writes multiple slog.Attr into the target which is either a zerolog.Context
// or a *zerolog.Event.
func mapAttrs[T zlogWriter[T]](target T, a ...slog.Attr) T {
	for _, attr := range a {
		target = mapAttr(target, attr)
	}
	return target
}

// mapAttr writes slog.Attr into the target which is either a zerolog.Context
// or a *zerolog.Event.
func mapAttr[T zlogWriter[T]](target T, a slog.Attr) T {
	value := a.Value.Resolve()
	switch value.Kind() {
	case slog.KindGroup:
		return target.Dict(a.Key, mapAttrs(zerolog.Dict(), value.Group()...))
	case slog.KindBool:
		return target.Bool(a.Key, value.Bool())
	case slog.KindDuration:
		return target.Dur(a.Key, value.Duration())
	case slog.KindFloat64:
		return target.Float64(a.Key, value.Float64())
	case slog.KindInt64:
		return target.Int64(a.Key, value.Int64())
	case slog.KindString:
		return target.Str(a.Key, value.String())
	case slog.KindTime:
		return target.Time(a.Key, value.Time())
	case slog.KindUint64:
		return target.Uint64(a.Key, value.Uint64())
	case slog.KindAny:
		fallthrough
	default:
		return mapAttrAny(target, a.Key, value.Any())
	}
}

func mapAttrAny[T zlogWriter[T]](target T, key string, value any) T {
	switch v := value.(type) {
	case net.IP:
		return target.IPAddr(key, v)
	case net.IPNet:
		return target.IPPrefix(key, v)
	case net.HardwareAddr:
		return target.MACAddr(key, v)
	case error:
		return target.AnErr(key, v)
	case fmt.Stringer:
		return target.Stringer(key, v)
	case json.Marshaler:
		txt, err := v.MarshalJSON()
		if err == nil {
			return target.RawJSON(key, txt)
		}
		return target.Str(key, "!ERROR:"+err.Error())
	case encoding.TextMarshaler:
		txt, err := v.MarshalText()
		if err == nil {
			return target.Str(key, string(txt))
		}
		return target.Str(key, "!ERROR:"+err.Error())
	default:
		return target.Interface(key, value)
	}
}

// zerologLevel maps slog.Level into zerolog.Level.
func zerologLevel(lvl slog.Level) zerolog.Level {
	switch {
	case lvl < slog.LevelDebug:
		return zerolog.TraceLevel
	case lvl < slog.LevelInfo:
		return zerolog.DebugLevel
	case lvl < slog.LevelWarn:
		return zerolog.InfoLevel
	case lvl < slog.LevelError:
		return zerolog.WarnLevel
	default:
		return zerolog.ErrorLevel
	}
}
