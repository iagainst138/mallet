package mallet

import (
	"context"
	"errors"
	"log/slog"
	"os"
)

// TeeHandler distributes log records to multiple slog.Handlers.
type TeeHandler struct {
	handlers []slog.Handler
}

// NewTeeHandler creates a new TeeHandler combining the provided handlers.
func NewTeeHandler(handlers ...slog.Handler) *TeeHandler {
	return &TeeHandler{handlers: handlers}
}

// Enabled reports whether the logger handles records at the given level.
func (t *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle dispatches the record to all embedded handlers.
func (t *TeeHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range t.handlers {
		// Only send to the handler if it's enabled for this level
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	// errors.Join requires Go 1.20+
	return errors.Join(errs...)
}

// WithAttrs returns a new TeeHandler with additional attributes applied to all handlers.
func (t *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		handlers[i] = h.WithAttrs(attrs)
	}
	return &TeeHandler{handlers: handlers}
}

// WithGroup returns a new TeeHandler with a group name applied to all handlers.
func (t *TeeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		handlers[i] = h.WithGroup(name)
	}
	return &TeeHandler{handlers: handlers}
}

// SetupDualLogger initializes a logger that writes text to stdout and JSON to a file.
func SetupDualLogger(logFilePath string) (*slog.Logger, *os.File, error) {
	// 1. Open the file for JSON logging
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, nil, err
	}

	// 2. Create the JSON handler for the file
	jsonHandler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug, // We can capture all logs in the file
	})

	// 3. Create the Text handler for the console
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo, // We might only want Info and above on the console
	})

	// 4. Combine them using our TeeHandler
	tee := NewTeeHandler(textHandler, jsonHandler)

	// 5. Create the final logger
	logger := slog.New(tee)

	return logger, file, nil
}

// TODO integrate this

func main() {
	// Initialize our dual logger
	logger, file, err := SetupDualLogger("application.jsonl")
	if err != nil {
		panic(err)
	}
	// Make sure to close the file when the application exits
	defer file.Close()

	// (Optional) Set it as the global default logger
	slog.SetDefault(logger)

	// --- Usage Examples ---

	// This will show up in both the console (Plain Text) and the file (JSON)
	slog.Info("Application started", "version", "1.0.0", "port", 8080)

	// Because of the configuration in SetupDualLogger, this Debug message
	// will ONLY be written to the JSON file, keeping your console clean.
	slog.Debug("Connecting to database...", "host", "localhost", "retries", 3)

	// Errors show up in both
	slog.Error("Failed to load config", "err", errors.New("file not found"))
}
