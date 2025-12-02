package false_positive

import "time"

// TwoGoroutinesCommunicating spawns two goroutines that can communicate.
// This is not a real leak because G2 and G3 will eventually synchronize.
// GoLeak's retry mechanism (~430ms) gives G3 time to run, so no false positive.
func TwoGoroutinesCommunicating() {
	c := make(chan int)
	go func() { <-c }()    // G2 - receiver
	go func() { c <- 1 }() // G3 - sender
}

// TwoGoroutinesSlowSender is a variant where G3 does slow work before sending.
// If delay exceeds GoLeak's retry window (~430ms), GoLeak reports a false positive.
func TwoGoroutinesSlowSender(delay time.Duration) {
	c := make(chan int)
	go func() { <-c }()
	go func() {
		time.Sleep(delay)
		c <- 1
	}()
}
