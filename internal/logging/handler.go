// Package logging provides a context-aware slog handler that automatically
// extracts request-scoped attributes from the context and appends them to
// every log record. This gives all log lines within an HTTP request a shared
// correlation ID without callers needing to pass it explicitly.
//
// Two layers of context are supported:
//
//  1. ContextAttrs — set once by the logging middleware with the request ID.
//  2. Extractors — optional functions registered at init time that pull
//     additional fields (user_id, tenant_id) from context. This avoids an
//     import dependency from logging → auth while still enriching every log
//     line with identity information set by later middleware.
package logging

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// ContextAttrs are the request-scoped attributes injected by the logging
// middleware and automatically attached to every slog record.
type ContextAttrs struct {
	RequestID string
}

// WithAttrs stores request-scoped logging attributes in the context.
func WithAttrs(ctx context.Context, attrs ContextAttrs) context.Context {
	return context.WithValue(ctx, contextKey{}, attrs)
}

// AttrsFromContext retrieves the request-scoped logging attributes, or returns
// a zero value if none are set.
func AttrsFromContext(ctx context.Context) ContextAttrs {
	v, _ := ctx.Value(contextKey{}).(ContextAttrs)
	return v
}

// Extractor pulls a named attribute value from the context. Registered
// extractors are called on every log record to enrich it with dynamic
// context values (e.g. user_id set by auth middleware after the logging
// middleware has already run).
type Extractor struct {
	Key     string
	Extract func(ctx context.Context) string
}

// ContextHandler wraps an slog.Handler and enriches every log record with
// request-scoped attributes from the context.
type ContextHandler struct {
	inner      slog.Handler
	extractors []Extractor
}

// NewContextHandler creates a new ContextHandler wrapping the given handler.
// Optional extractors are called on every log record to pull additional
// attributes from context (e.g. user_id, tenant_id from auth middleware).
func NewContextHandler(inner slog.Handler, extractors ...Extractor) *ContextHandler {
	return &ContextHandler{inner: inner, extractors: extractors}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, record slog.Record) error {
	attrs := AttrsFromContext(ctx)
	if attrs.RequestID != "" {
		record.AddAttrs(slog.String("request_id", attrs.RequestID))
	}
	for _, ext := range h.extractors {
		if v := ext.Extract(ctx); v != "" {
			record.AddAttrs(slog.String(ext.Key, v))
		}
	}
	return h.inner.Handle(ctx, record)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs), extractors: h.extractors}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name), extractors: h.extractors}
}
