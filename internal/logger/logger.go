// Package logger control plane (API aur worker) ke liye ek hi jagah se slog
// banata hai, taki dono binaries ka output ek jaisa dikhe.
package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

const EnvProduction = "production"

const (
	// FormatAuto ENV dekh kar chunta hai: production me compact JSON,
	// development me indented JSON.
	FormatAuto = "auto"

	// FormatJSON: ek record = ek line. Log aggregators (loki, cloudwatch)
	// line ke hisaab se hi parse karte hain, isliye production me yahi.
	FormatJSON = "json"

	// FormatPretty: wahi JSON, par indent ke saath - har field apni line pe.
	// Padhne ke liye, machine ke liye nahi.
	FormatPretty = "pretty"

	// FormatText: tint ka rangeen text. JSON ka shape nahi chahiye to.
	FormatText = "text"
)

// ANSI codes JSON string value ke andar jaate hain - output valid JSON bhi
// rehta hai aur terminal me rangeen bhi dikhta hai. Production me colours off,
// warna journald me raw escape bytes bach jaate hain.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiGreen  = "\033[32m"
	ansiCyan   = "\033[36m"
)

// Options config se aata hai. Yahan plain strings isliye hain taki ye package
// internal/config pe nirbhar na ho - agentd bhi isi ko use kar sake.
type Options struct {
	Env    string // development | production
	Level  string // debug | info | warn | error ("" = env ke hisaab se)
	Format string // auto | json | pretty | text
}

// New Options ke hisaab se logger banata hai.
//
// Default (Format khali ya "auto"):
//
//	development -> indented JSON, rangeen
//	production  -> compact JSON, ek record ek line
func New(opts Options) *slog.Logger {
	prod := isProduction(opts.Env)
	level := parseLevel(opts.Level, prod)
	colour := !prod

	// source file:line sirf debug pe - har line pe ye dena mehenga bhi hai
	// aur output ko bhar bhi deta hai.
	addSource := level <= slog.LevelDebug

	format := resolveFormat(opts.Format, prod)

	if format == FormatText {
		return slog.New(tint.NewTextHandler(os.Stdout, &tint.Options{
			Level:       level,
			AddSource:   addSource,
			NoColor:     !colour,
			TimeFormat:  "15:04:05.000",
			ReplaceAttr: textAttr,
		}))
	}

	return slog.New(slog.NewJSONHandler(
		output(colour, format == FormatPretty),
		&slog.HandlerOptions{
			Level:       level,
			AddSource:   addSource,
			ReplaceAttr: jsonAttr(colour),
		},
	))
}

// jsonWriter JSONHandler ke aur stdout ke beech me baithta hai.
//
// Ye kaam yahan isliye hota hai, handler me nahi: JSONHandler har record ke
// liye theek ek baar Write karta hai, poora record ek hi buffer me - aur wo
// call apne mutex ke andar hota hai. Isliye yahan ek complete JSON object
// milta hai aur do records aapas me gaddmadd nahi hote.
type jsonWriter struct {
	out    io.Writer
	indent bool
	colour bool
}

func (w jsonWriter) Write(p []byte) (int, error) {
	b := p

	if w.indent {
		var buf bytes.Buffer
		// TrimRight isliye ki handler ka trailing \n indent ke andar na jaye
		if err := json.Indent(&buf, bytes.TrimRight(p, "\n"), "", "  "); err != nil {
			// JSON nahi hai (aisa hona nahi chahiye) - jaisa aaya waisa likh do
			return w.out.Write(p)
		}
		buf.WriteByte('\n')
		b = buf.Bytes()
	}

	if w.colour {
		// encoding/json har control character escape kar deta hai, isliye
		// humara ESC (0x1b) wire pe literal  ban jaata hai aur terminal
		// use rang nahi samajhta. Yahan wo wapas asli byte ban jaata hai.
		b = bytes.ReplaceAll(b, jsonEscapedESC, rawESC)
	}

	if _, err := w.out.Write(b); err != nil {
		return 0, err
	}

	// Caller ko wahi length chahiye jo usne di thi, warna short-write error.
	return len(p), nil
}

var (
	jsonEscapedESC = []byte{'\\', 'u', '0', '0', '1', 'b'}
	rawESC         = []byte{0x1b}
)

func output(colour, indent bool) io.Writer {
	if !colour && !indent {
		return os.Stdout
	}
	return jsonWriter{out: os.Stdout, indent: indent, colour: colour}
}

func resolveFormat(format string, prod bool) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatJSON:
		return FormatJSON
	case FormatPretty:
		return FormatPretty
	case FormatText:
		return FormatText
	default:
		if prod {
			return FormatJSON
		}
		return FormatPretty
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

// parseLevel khali value pe env ke hisaab se chunta hai: dev me debug chahiye,
// production me debug output ka kharcha bekar hai.
func parseLevel(s string, prod bool) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		if prod {
			return slog.LevelInfo
		}
		return slog.LevelDebug
	}
}
