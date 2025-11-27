package channel

import (
	"testing"

	"go.uber.org/goleak"
)

// TestFixedHandleRequest demonstrates proper cleanup with buffered channel.
func TestFixedHandleRequest(t *testing.T) {
	// GoLeak will verify no goroutines leaked
	defer goleak.VerifyNone(t)

	// Call with invalid item - this causes early return
	err := FixedHandleRequest(Item{ID: 1, Invalid: true})
	if err == nil {
		t.Fatal("expected error for invalid item")
	}

	// GoLeak verifies and finds that no goroutines are leaked
}

// TestFixedHandleRequest_ValidItem tests the happy path
func TestFixedHandleRequest_ValidItem(t *testing.T) {
	defer goleak.VerifyNone(t)

	// call with valid item
	err := FixedHandleRequest(Item{ID: 1, Invalid: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// also no leaks here
}
