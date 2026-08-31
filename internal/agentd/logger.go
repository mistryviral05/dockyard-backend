package agentd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

const (
	EnvProduction = "production"

	LogFormatAuto = "auto"
	LogFormatText = "text"
	LogFormatJSON = "json"
)

// ANSI codes. JSON me ye string value ke andar jaate hain - valid
// JSON hai, isliye output parse bhi hota hai aur terminal me rangeen bhi dikhta
// hai. Production me colours off rehte hain, warna journald me raw bytes bachte
// hain.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiGreen  = "\033[32m"
	ansiCyan   = "\033[36m"
)

// NewLogger format ENV se chunta hai, output se nahi:
//
//   - development : tint ka rangeen text
//   - production  : JSON, bina colour ke
//
// DOCKYARD_LOG_FORMAT se ise override kar sakte ho - dev me JSON dekhna ho to
// format=json karo, colours phir bhi bane rahenge.
func NewLogger(cfg *Config) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	prod := isProduction(cfg.Env)
	colour := !prod


	addSource := level <= slog.LevelDebug

	var h slog.Handler
	if resolveFormat(cfg.LogFormat, prod) == LogFormatJSON {
		h = slog.NewJSONHandler(jsonOut(colour), &slog.HandlerOptions{
			Level:       level,
			AddSource:   addSource,
			ReplaceAttr: jsonAttr(colour),
		})
	} else {
		h = tint.NewTextHandler(os.Stdout, &tint.Options{
			Level:     level,
			AddSource: addSource,
			NoColor:   !colour,

			TimeFormat:  "15:04:05.000",
			ReplaceAttr: textAttr,
		})
	}


	return slog.New(h).With(slog.String("version", Version))
}

func resolveFormat(format string, prod bool) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case LogFormatJSON:
		return LogFormatJSON
	case LogFormatText:
		return LogFormatText
	default:
		if prod {
			return LogFormatJSON
		}
		return LogFormatText
	}
}

func isProduction(env string) bool {
	e := strings.ToLower(strings.TrimSpace(env))
	return e == EnvProduction || e == "prod"
}


func textAttr(groups []string, a slog.Attr) slog.Attr {
	if isErrorAttr(a) {
		return tint.Attr(9, a)
	}
	return a
}


func jsonAttr(colour bool) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) > 0 {
			return a
		}

		switch {
		case a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime:
			return slog.String(slog.TimeKey, a.Value.Time().UTC().Format(time.RFC3339Nano))

		case colour && a.Key == slog.LevelKey:
			lvl, ok := a.Value.Any().(slog.Level)
			if !ok {
				return a
			}
			return slog.String(slog.LevelKey, paint(levelColour(lvl), lvl.String()))

		case colour && isErrorAttr(a):
			return slog.String(a.Key, paint(ansiRed, valueString(a.Value)))
		}

		return a
	}
}

// encoding/json har control character ko escape kar deta hai, isliye humara
// ESC (0x1b) wire pe literal  ban kar jaata hai aur terminal use rang
// nahi samajhta. Ye writer wahi escape wapas asli byte me badal deta hai.
//
// Sirf dev me lagta hai. Strict JSON string ke andar raw control character
// allow nahi karta, isliye production (colour off) me ye writer beech me aata
// hi nahi - wahan output bilkul saaf JSON rehta hai.
func jsonOut(colour bool) io.Writer {
	if !colour {
		return os.Stdout
	}
	return unescapeANSIWriter{w: os.Stdout}
}

var (
	jsonEscapedESC = []byte{'\\', 'u', '0', '0', '1', 'b'}
	rawESC         = []byte{0x1b}
)

type unescapeANSIWriter struct{ w io.Writer }

func (u unescapeANSIWriter) Write(p []byte) (int, error) {
	if _, err := u.w.Write(bytes.ReplaceAll(p, jsonEscapedESC, rawESC)); err != nil {
		return 0, err
	}
	// Caller ko wahi length chahiye jo usne di thi, warna short-write error.
	return len(p), nil
}

func isErrorAttr(a slog.Attr) bool {
	if a.Key == "error" {
		return true
	}
	_, ok := a.Value.Any().(error)
	return ok
}

func valueString(v slog.Value) string {
	if err, ok := v.Any().(error); ok {
		return err.Error()
	}
	return v.String()
}

func paint(colour, s string) string {
	return colour + s + ansiReset
}

func levelColour(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return ansiRed
	case l >= slog.LevelWarn:
		return ansiYellow
	case l >= slog.LevelInfo:
		return ansiGreen
	default:
		return ansiCyan
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
