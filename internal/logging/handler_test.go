package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestContextHandler_AddsRequestID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(inner)
	logger := slog.New(handler)

	ctx := WithAttrs(context.Background(), ContextAttrs{RequestID: "req-123"})
	logger.InfoContext(ctx, "test message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if entry["request_id"] != "req-123" {
		t.Errorf("request_id = %v, want req-123", entry["request_id"])
	}
	if entry["msg"] != "test message" {
		t.Errorf("msg = %v, want test message", entry["msg"])
	}
}

func TestContextHandler_NoAttrsWithoutContext(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(inner)
	logger := slog.New(handler)

	logger.InfoContext(context.Background(), "no context")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if _, ok := entry["request_id"]; ok {
		t.Error("request_id should not be present without context attrs")
	}
}

func TestContextHandler_PreservesExplicitAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(inner)
	logger := slog.New(handler)

	ctx := WithAttrs(context.Background(), ContextAttrs{RequestID: "req-789"})
	logger.InfoContext(ctx, "with extra", "custom_field", "custom_value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if entry["request_id"] != "req-789" {
		t.Errorf("request_id = %v, want req-789", entry["request_id"])
	}
	if entry["custom_field"] != "custom_value" {
		t.Errorf("custom_field = %v, want custom_value", entry["custom_field"])
	}
}

type testContextKey struct{ name string }

func TestContextHandler_Extractors(t *testing.T) {
	userKey := testContextKey{"user"}
	tenantKey := testContextKey{"tenant"}

	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(inner,
		Extractor{Key: "user_id", Extract: func(ctx context.Context) string {
			v, _ := ctx.Value(userKey).(string)
			return v
		}},
		Extractor{Key: "tenant_id", Extract: func(ctx context.Context) string {
			v, _ := ctx.Value(tenantKey).(string)
			return v
		}},
	)
	logger := slog.New(handler)

	ctx := WithAttrs(context.Background(), ContextAttrs{RequestID: "req-ext"})
	ctx = context.WithValue(ctx, userKey, "user-1")
	ctx = context.WithValue(ctx, tenantKey, "tenant-a")
	logger.InfoContext(ctx, "with extractors")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if entry["request_id"] != "req-ext" {
		t.Errorf("request_id = %v, want req-ext", entry["request_id"])
	}
	if entry["user_id"] != "user-1" {
		t.Errorf("user_id = %v, want user-1", entry["user_id"])
	}
	if entry["tenant_id"] != "tenant-a" {
		t.Errorf("tenant_id = %v, want tenant-a", entry["tenant_id"])
	}
}

func TestContextHandler_ExtractorSkipsEmpty(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(inner,
		Extractor{Key: "user_id", Extract: func(ctx context.Context) string {
			return "" // no value
		}},
	)
	logger := slog.New(handler)

	ctx := WithAttrs(context.Background(), ContextAttrs{RequestID: "req-empty"})
	logger.InfoContext(ctx, "empty extractor")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if entry["request_id"] != "req-empty" {
		t.Errorf("request_id = %v, want req-empty", entry["request_id"])
	}
	if _, ok := entry["user_id"]; ok {
		t.Error("user_id should not be present when extractor returns empty string")
	}
}

func TestContextHandler_WithAttrsPreservesExtractors(t *testing.T) {
	userKey := testContextKey{"user"}

	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, nil)
	handler := NewContextHandler(inner,
		Extractor{Key: "user_id", Extract: func(ctx context.Context) string {
			v, _ := ctx.Value(userKey).(string)
			return v
		}},
	)

	// WithAttrs creates a child handler — extractors should carry over
	childHandler := handler.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(childHandler)

	ctx := WithAttrs(context.Background(), ContextAttrs{RequestID: "req-child"})
	ctx = context.WithValue(ctx, userKey, "user-child")
	logger.InfoContext(ctx, "child handler")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if entry["request_id"] != "req-child" {
		t.Errorf("request_id = %v, want req-child", entry["request_id"])
	}
	if entry["user_id"] != "user-child" {
		t.Errorf("user_id = %v, want user-child", entry["user_id"])
	}
	if entry["component"] != "test" {
		t.Errorf("component = %v, want test", entry["component"])
	}
}
