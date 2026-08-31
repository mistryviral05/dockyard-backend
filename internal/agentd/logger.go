package agentd

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)


func NewLogger(cfg *Config) *slog.Logger {
	return slog.New(
		tint.NewTextHandler(os.Stdout, &tint.Options{
			Level:      parseLevel(cfg.LogLevel),
			TimeFormat: time.Kitchen,
		}),
	)
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
