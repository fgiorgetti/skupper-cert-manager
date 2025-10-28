package logger

import (
	"log/slog"
	"os"
)

func NewLogger(component, namespace string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})
	return slog.New(handler).With(
		"component", component,
		"namespace", namespace)
}
