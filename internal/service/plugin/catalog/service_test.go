package catalog

import (
	"context"
	"testing"
)

func TestListAndGet(t *testing.T) {
	svc := NewService("")
	entries, err := svc.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if entries == nil {
		t.Fatal("expected non-nil slice")
	}
	_, err = svc.Get(context.Background(), "missing/plugin")
	if err != ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
