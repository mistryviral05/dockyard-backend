package logger

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func New() *slog.Logger {
	return slog.New(
		tint.NewTextHandler(os.Stdout, &tint.Options{
			Level:      slog.LevelDebug,
			TimeFormat: time.Kitchen,
		}),
	)
}
