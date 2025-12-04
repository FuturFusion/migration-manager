package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
)

type Logger interface {
	Debug(msg string, args ...any)
	DebugContext(ctx context.Context, msg string, args ...any)
	Error(msg string, args ...any)
	ErrorContext(ctx context.Context, msg string, args ...any)
	Info(msg string, args ...any)
	InfoContext(ctx context.Context, msg string, args ...any)
	Warn(msg string, args ...any)
	WarnContext(ctx context.Context, msg string, args ...any)
	With(args ...any) *slog.Logger
}

func InitLogger(filepath string, verbose bool, debug bool) (*slog.LevelVar, error) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}

	if debug {
		level = slog.LevelDebug
	}

	var writer io.Writer = os.Stderr

	if filepath != "" {
		f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return nil, err
		}

		writer = io.MultiWriter(writer, f)
	}

	var handler slog.LevelVar
	handler.Set(level)

	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: &handler,
		// Add source information, if debug level is enabled.
		AddSource: debug,
	}))

	slog.SetDefault(logger)

	return &handler, nil
}

// Err is a helper function to ensure errors are always logged with the key
// "err". Additionally this becomes the single point in code, where we could
// tweak how errors are logged, e.g. to handle application specific error types
// or to add stack trace information in debug mode.
func Err(err error) slog.Attr {
	return slog.Any("err", err)
}

func ValidateLevel(levelStr string) error {
	validLogLevels := []string{slog.LevelDebug.String(), slog.LevelInfo.String(), slog.LevelWarn.String(), slog.LevelError.String()}
	if !slices.Contains(validLogLevels, levelStr) {
		return fmt.Errorf("Log level %q is invalid, must be one of %q", levelStr, strings.Join(validLogLevels, ","))
	}

	return nil
}

func ParseLevel(levelStr string) slog.Level {
	level := slog.LevelWarn
	switch levelStr {
	case slog.LevelDebug.String():
		level = slog.LevelDebug
	case slog.LevelInfo.String():
		level = slog.LevelInfo
	case slog.LevelWarn.String():
		level = slog.LevelWarn
	case slog.LevelError.String():
		level = slog.LevelError
	}

	return level
}
