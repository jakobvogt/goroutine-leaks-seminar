package false_positive

import (
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestTwoGoroutinesCommunicating: GoLeak's retry handles this, no false positive
func TestTwoGoroutinesCommunicating(t *testing.T) {
	defer goleak.VerifyNone(t)
	TwoGoroutinesCommunicating()
}

// TestSlowSenderUnderTimeout: 100ms is under the ~430ms retry window, passes
func TestSlowSenderUnderTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	TwoGoroutinesSlowSender(100 * time.Millisecond)
}

// TestSlowSenderOverTimeout: 1s exceeds retry window => false positive
// GoLeak reports a leak, but G2 and G3 can still communicate once sleep ends
func TestSlowSenderOverTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	TwoGoroutinesSlowSender(1 * time.Second)
}
