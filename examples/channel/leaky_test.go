package channel

import (
	"testing"

	"go.uber.org/goleak"
)

func TestLeakyHandleRequest(t *testing.T) {
	// GoLeak will verify no goroutines leaked
	defer goleak.VerifyNone(t)

	// Call with invalid item -> this causes early return and goroutine leak
	err := LeakyHandleRequest(Item{ID: 1, Invalid: true})
	if err == nil {
		t.Fatal("expected error for invalid item")
	}

	// Test ends here, but the goroutine is still blocked trying to send!
	// GoLeak will detect this and fail the test.
}
