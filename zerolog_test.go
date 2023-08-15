package zeroslog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"maps"

	"testing/slogtest"

	"github.com/rs/zerolog"
)

type stringer struct{}

func (stringer) String() string {
	return "stringer"
}

type marshaller struct{ err error }

func (m marshaller) MarshalText() (text []byte, err error) {
	return []byte("marshaller"), m.err
}

type jsoner struct {
	foo string
	err error
}

func (j jsoner) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"foo": %q}`, j.foo)), j.err
}

type unknown struct{ Foo string }

var (
	now = time.Now()

	attrs = []slog.Attr{
		slog.String("titi", "toto"),
		slog.String("tata", "tutu"),
		slog.Int("foo", 12),
		slog.Uint64("bar", 42),
		slog.Duration("dur", 3*time.Second),
		slog.Bool("bool", true),
		slog.Float64("float", 23.7),
		slog.Time("thetime", now),
		slog.Any("err", errors.New("yo")),
		slog.Group("empty"),
		slog.Group("group", slog.String("bar", "baz")),
		slog.Any("ip", net.IP{192, 168, 1, 2}),
		slog.Any("ipnet", net.IPNet{IP: net.IP{192, 168, 1, 0}, Mask: net.IPv4Mask(255, 255, 255, 0)}),
		slog.Any("mac", net.HardwareAddr{0x00, 0x00, 0x5e, 0x00, 0x53, 0x01}),
		slog.Any("stringer", stringer{}),
		slog.Any("marshaller", &marshaller{}),
		slog.Any("marshaller-err", &marshaller{err: errors.New("failure")}),
		slog.Any("unknown", unknown{Foo: "bar"}),
		slog.Any("json", &jsoner{foo: "bar"}),
		slog.Any("json-err", &jsoner{err: errors.New("failure")}),
	}

	exp = map[string]any{
		"titi":           "toto",
		"tata":           "tutu",
		"foo":            12.0,
		"bar":            42.0,
		"dur":            3000.0,
		"bool":           true,
		"float":          23.7,
		"thetime":        now.Format(time.RFC3339),
		"err":            "yo",
		"group":          map[string]any{"bar": "baz"},
		"ip":             "192.168.1.2",
		"ipnet":          "192.168.1.0/24",
		"mac":            "00:00:5e:00:53:01",
		"stringer":       "stringer",
		"marshaller":     "marshaller",
		"marshaller-err": "!ERROR:failure",
		"unknown":        map[string]any{"Foo": "bar"},
		"json":           map[string]any{"foo": "bar"},
		"json-err":       "!ERROR:failure",
	}

	levels = []struct {
		zlvl zerolog.Level
		slvl slog.Level
	}{
		{zerolog.TraceLevel, slog.LevelDebug - 1},
		{zerolog.DebugLevel, slog.LevelDebug},
		{zerolog.InfoLevel, slog.LevelInfo},
		{zerolog.WarnLevel, slog.LevelWarn},
		{zerolog.WarnLevel, slog.LevelWarn + 1},
		{zerolog.WarnLevel, slog.LevelError - 1},
		{zerolog.ErrorLevel, slog.LevelError},
		{zerolog.ErrorLevel, slog.LevelError + 1},
	}
)

func TestZerolog_Levels(t *testing.T) {
	out := bytes.Buffer{}
	for _, lvl := range levels {
		t.Run(lvl.slvl.String(), func(t *testing.T) {
			hdl := NewZerologJsonHandler(&out, &HandlerOptions{Level: lvl.slvl})
			for _, l := range levels {
				enabled := l.slvl >= lvl.slvl
				if hdl.Enabled(nil, l.slvl) != enabled {
					t.Fatalf("Level %s enablement status unexpected", l.slvl)
				}
				hdl.Handle(nil, slog.NewRecord(time.Now(), l.slvl, "foobar", 0))
				if enabled {
					m := map[string]any{}
					if err := json.NewDecoder(&out).Decode(&m); err != nil {
						t.Fatalf("Failed to json decode log output: %s", err.Error())
					}
					if m[zerolog.LevelFieldName] != l.zlvl.String() {
						t.Fatalf("Unexpectd value for field %s. Got %s but expectd %s", zerolog.LevelFieldName, m[zerolog.LevelFieldName], l.zlvl.String())
					}
				}
			}

		})
	}
}

func TestZerolog_Levels_NoOption(t *testing.T) {
	out := bytes.Buffer{}
	for _, lvl := range levels {
		t.Run(lvl.slvl.String(), func(t *testing.T) {
			hdl := NewZerologHandler(zerolog.New(&out).Level(lvl.zlvl), nil)
			for _, l := range levels {
				enabled := l.zlvl >= lvl.zlvl
				if hdl.Enabled(nil, l.slvl) != enabled {
					t.Fatalf("Level %s enablement status unexpected", l.slvl)
				}
				hdl.Handle(nil, slog.NewRecord(time.Now(), l.slvl, "foobar", 0))
				m := map[string]any{}
				err := json.NewDecoder(&out).Decode(&m)
				if enabled {
					if err != nil {
						t.Fatalf("Failed to json decode log output: %s", err.Error())
					}
					if m[zerolog.LevelFieldName] != l.zlvl.String() {
						t.Fatalf("Unexpectd value for field %s. Got %s but expectd %s", zerolog.LevelFieldName, m[zerolog.LevelFieldName], l.zlvl.String())
					}
				} else {
					if !errors.Is(err, io.EOF) {
						t.Fatalf("Expected io.EOF error but got %s", err)
					}
				}
			}

		})
	}
}

func TestZerolog_NoGroup(t *testing.T) {
	out := bytes.Buffer{}
	hdl := NewZerologJsonHandler(&out, nil).
		WithAttrs([]slog.Attr{slog.String("attr", "the attr")})

	if !hdl.Enabled(nil, slog.LevelError) {
		t.Errorf("Level %s must be enabled", slog.LevelError)
	}
	if hdl.Enabled(nil, slog.LevelDebug) {
		t.Errorf("Level %s must be disabled", slog.LevelDebug)
	}

	rec := slog.NewRecord(now, slog.LevelError, "foobar", 0)
	rec.AddAttrs(attrs...)
	hdl.Handle(nil, rec)

	expected := maps.Clone(exp)
	expected[zerolog.LevelFieldName] = zerolog.LevelErrorValue
	expected[zerolog.MessageFieldName] = "foobar"
	expected[zerolog.TimestampFieldName] = now.Format(time.RFC3339)
	expected["attr"] = "the attr"

	m := map[string]any{}
	if err := json.NewDecoder(&out).Decode(&m); err != nil {
		t.Fatalf("Failed to json decode log output: %s", err.Error())
	}
	if !reflect.DeepEqual(expected, m) {
		t.Fatalf("Unexpected fields. Got %v, expected %v", m, expected)
	}
}

func TestZerolog_Group(t *testing.T) {
	out := bytes.Buffer{}
	hdl := NewZerologJsonHandler(&out, nil).
		WithAttrs([]slog.Attr{slog.String("attr", "the attr")}).
		WithGroup("testgroup").
		WithAttrs([]slog.Attr{slog.String("attr", "the attr")}).
		WithGroup("subgroup")

	if !hdl.Enabled(nil, slog.LevelError) {
		t.Errorf("Level %s must be enabled", slog.LevelError)
	}
	if hdl.Enabled(nil, slog.LevelDebug) {
		t.Errorf("Level %s must be disabled", slog.LevelDebug)
	}

	rec := slog.NewRecord(now, slog.LevelWarn, "foobar", 0)
	rec.AddAttrs(attrs...)
	hdl.Handle(nil, rec)

	expected := map[string]any{
		zerolog.LevelFieldName:     zerolog.LevelWarnValue,
		zerolog.MessageFieldName:   "foobar",
		zerolog.TimestampFieldName: now.Format(time.RFC3339),
		"attr":                     "the attr",
		"testgroup": map[string]any{
			"attr":     "the attr",
			"subgroup": maps.Clone(exp),
		},
	}

	m := map[string]any{}
	if err := json.NewDecoder(&out).Decode(&m); err != nil {
		t.Fatalf("Failed to json decode log output: %s", err.Error())
	}
	if !reflect.DeepEqual(expected, m) {
		t.Fatalf("Unexpected fields. Got %v, expected %v", m, expected)
	}
}

func TestZerolog_AddSource(t *testing.T) {
	out := bytes.Buffer{}
	hdl := NewZerologJsonHandler(&out, &HandlerOptions{AddSource: true})
	pc, file, line, _ := runtime.Caller(0)
	hdl.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "foobar", pc))
	m := map[string]any{}
	if err := json.NewDecoder(&out).Decode(&m); err != nil {
		t.Fatalf("Failed to json decode log output: %s", err.Error())
	}
	if m[zerolog.CallerFieldName].(string) != fmt.Sprintf("%s:%d", file, line) {
		t.Fatalf("Unexpected field %s: %s", zerolog.CallerFieldName, m[zerolog.CallerFieldName].(string))
	}
}

func TestZerolog_ConsoleHandler(t *testing.T) {
	out := bytes.Buffer{}
	hdl := NewZerologConsoleHandler(&out, nil)
	hdl.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "foobar", 0))
	txt := out.String()
	if !strings.Contains(txt, "foobar") || !strings.Contains(txt, "INF") {
		t.Errorf("Unexpected console output %q", txt)
	}
}

// TestHandler uses slogtest.TestHandler from stdlib to validate
// the zerolog handler implementation.
func TestHandler(t *testing.T) {
	out := bytes.Buffer{}
	dec := json.NewDecoder(&out)
	hdl := NewZerologJsonHandler(&out, &HandlerOptions{Level: slog.LevelDebug})
	err := slogtest.TestHandler(hdl, func() []map[string]any {
		results := []map[string]any{}
		m := map[string]any{}
		for dec.Decode(&m) != io.EOF {
			results = append(results, m)
			m = map[string]any{}
		}
		return results
	})
	if err != nil {
		t.Fatal(err)
	}
}
