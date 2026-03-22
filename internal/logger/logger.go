package logger

import (
	"log/slog"
	"os"
)

// Init configures the global slog logger with JSON output.
// Every log line will include a "service" field automatically.
func Init(service string) {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(h).With("service", service))
}
