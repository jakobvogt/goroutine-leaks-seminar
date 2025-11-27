package timeout

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestLeakyFetchWithTimeout - this test PASSES but the bug is still there!
//
// GoLeak doesn't catch it because we only test the happy path
// (long timeout, fetch completes in time). The leak only happens
// when ctx actually times out, which rarely happens in tests.
func TestLeakyFetchWithTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)

	// long timeout so fetch completes before timeout (the "happy path")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := LeakyFetchWithTimeout(ctx, Item{ID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != 420 {
		t.Fatalf("expected 420, got %d", result.Value)
	}

	// GoLeak passes! Bug is still there, just wasn't triggered
}
