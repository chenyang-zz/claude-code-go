package tool

import (
	"context"
	"testing"
)

func TestWithProgress_GetProgress(t *testing.T) {
	var received any
	fn := ProgressFunc(func(data any) {
		received = data
	})

	ctx := WithProgress(context.Background(), fn)
	got := GetProgress(ctx)
	if got == nil {
		t.Fatal("GetProgress returned nil")
	}

	got("test-data")
	if received != "test-data" {
		t.Fatalf("expected received=%v, got %v", "test-data", received)
	}
}

func TestGetProgress_NilContext(t *testing.T) {
	got := GetProgress(nil)
	if got != nil {
		t.Fatal("expected nil for nil context")
	}
}

func TestGetProgress_NoCallback(t *testing.T) {
	got := GetProgress(context.Background())
	if got != nil {
		t.Fatal("expected nil when no callback set")
	}
}

func TestReportProgress_WithCallback(t *testing.T) {
	var received any
	fn := ProgressFunc(func(data any) {
		received = data
	})

	ctx := WithProgress(context.Background(), fn)
	ReportProgress(ctx, "hello")
	if received != "hello" {
		t.Fatalf("expected received=%v, got %v", "hello", received)
	}
}

func TestReportProgress_NoCallback(t *testing.T) {
	ctx := context.Background()
	ReportProgress(ctx, "should-not-panic")
}

func TestReportProgress_NilContext(t *testing.T) {
	ReportProgress(nil, "should-not-panic")
}
