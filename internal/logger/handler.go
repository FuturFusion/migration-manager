package logger

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

type Handler struct {
	*slog.LevelVar

	handlers []slog.Handler
	options  slog.HandlerOptions
}

// NewLogHandler creates a new log handler with the given default options, level, and sub-handlers.
func NewLogHandler(level slog.Level, options slog.HandlerOptions, handlers ...slog.Handler) *Handler {
	var leveler slog.LevelVar
	leveler.Set(level)
	if options.Level == nil {
		options.Level = &leveler
	}

	return &Handler{
		LevelVar: &leveler,
		options:  options,
		handlers: handlers,
	}
}

// Enabled returns true if any logger is enabled for the level.
func (h *Handler) Enabled(ctx context.Context, lvl slog.Level) bool {
	for _, h := range h.handlers {
		if h.Enabled(ctx, lvl) {
			return true
		}
	}

	return false
}

// Handle calls Handle for all handlers for which log level is enabled. Errors will be collected and returned together.
func (h *Handler) Handle(ctx context.Context, rec slog.Record) error {
	var errs []error
	wg := sync.WaitGroup{}
	for _, h := range h.handlers {
		if h.Enabled(ctx, rec.Level) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := h.Handle(ctx, rec.Clone())
				if err != nil {
					errs = append(errs, err)
				}
			}()
		}
	}

	wg.Wait()

	return errors.Join(errs...)
}

// WithAttrs returns a new MultiLog with WithAttrs called on all handlers.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	multi := &Handler{handlers: make([]slog.Handler, len(h.handlers))}
	for i := range h.handlers {
		multi.handlers[i] = h.handlers[i].WithAttrs(attrs)
	}

	return multi
}

// WithGroup returns a new MultiLog with WithGroup called on all handlers.
func (h *Handler) WithGroup(name string) slog.Handler {
	multi := &Handler{handlers: make([]slog.Handler, len(h.handlers))}
	for i := range h.handlers {
		multi.handlers[i] = h.handlers[i].WithGroup(name)
	}

	return multi
}
