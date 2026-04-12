package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"
)

// MaskingHandler is a custom slog.Handler that masks sensitive information like bot tokens.
type MaskingHandler struct {
	handler   slog.Handler
	botToken  string
	maskValue string
}

// NewMaskingHandler creates a new MaskingHandler.
func NewMaskingHandler(handler slog.Handler, botToken string) *MaskingHandler {
	return &MaskingHandler{
		handler:   handler,
		botToken:  botToken,
		maskValue: "***MASKED_TOKEN***",
	}
}

// Enabled implements slog.Handler.
func (h *MaskingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h *MaskingHandler) Handle(ctx context.Context, record slog.Record) error {
	// Mask token in the message
	msg := record.Message
	if strings.Contains(msg, h.botToken) {
		msg = strings.ReplaceAll(msg, h.botToken, h.maskValue)
	}

	newRecord := slog.NewRecord(record.Time, record.Level, msg, record.PC)
	record.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(h.maskAttr(a))
		return true
	})

	return h.handler.Handle(ctx, newRecord)
}

func (h *MaskingHandler) maskAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		val := a.Value.String()
		if strings.Contains(val, h.botToken) {
			return slog.String(a.Key, strings.ReplaceAll(val, h.botToken, h.maskValue))
		}
	} else if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		newAttrs := make([]any, len(attrs))
		for i, gAttr := range attrs {
			newAttrs[i] = h.maskAttr(gAttr)
		}
		return slog.Group(a.Key, newAttrs...)
	}
	return a
}

// WithAttrs implements slog.Handler.
func (h *MaskingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MaskingHandler{
		handler:   h.handler.WithAttrs(attrs),
		botToken:  h.botToken,
		maskValue: h.maskValue,
	}
}

// WithGroup implements slog.Handler.
func (h *MaskingHandler) WithGroup(name string) slog.Handler {
	return &MaskingHandler{
		handler:   h.handler.WithGroup(name),
		botToken:  h.botToken,
		maskValue: h.maskValue,
	}
}

// Setup creates and returns a configured logger.
func Setup(botToken string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	baseHandler := slog.NewTextHandler(os.Stdout, opts)

	var handler slog.Handler = baseHandler
	if botToken != "" {
		handler = NewMaskingHandler(baseHandler, botToken)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// WithTraceID adds a unique trace_id to the logger.
func WithTraceID(logger *slog.Logger) *slog.Logger {
	b := make([]byte, 8) // 64-bit random id is enough for tracing
	_, err := rand.Read(b)
	if err != nil {
		return logger.With("trace_id", "unknown")
	}
	traceID := hex.EncodeToString(b)
	return logger.With("trace_id", traceID)
}

// TokenMaskWriter is an io.Writer that masks a token before writing to the underlying logger.
type TokenMaskWriter struct {
	logger *slog.Logger
	token  string
}

func NewTokenMaskWriter(logger *slog.Logger, token string) *TokenMaskWriter {
	return &TokenMaskWriter{
		logger: logger,
		token:  token,
	}
}

func (w *TokenMaskWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if w.token != "" {
		msg = strings.ReplaceAll(msg, w.token, "***MASKED_TOKEN***")
	}
	// Log clean string to slog
	w.logger.Info(strings.TrimSpace(msg), "source", "tgbotapi")
	return len(p), nil
}
