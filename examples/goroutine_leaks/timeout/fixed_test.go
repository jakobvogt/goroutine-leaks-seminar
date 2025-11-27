package timeout

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestFixedFetchWithTimeout_Timeout verifies the buffered channel fix works
func TestFixedFetchWithTimeout_Timeout(t *testing.T) {
	defer goleak.VerifyNone(t)

	// ctx times out before fetch completes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := FixedFetchWithTimeout(ctx, Item{ID: 42})

	if err == nil {
		t.Fatalf("expected timeout error, got result: %v", result)
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // let goroutine finish

	// GoLeak passes, no leak!
}

// TestFixedFetchWithTimeout_Success - normal operation still works
func TestFixedFetchWithTimeout_Success(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, err := FixedFetchWithTimeout(ctx, Item{ID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value != 420 {
		t.Fatalf("expected 420, got %d", result.Value)
	}
}
