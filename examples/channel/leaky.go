package channel

import "errors"

// Item represents some data to process
type Item struct {
	ID      int
	Invalid bool
}

// Result represents the computation result
type Result struct {
	Value int
}

// LeakyHandleRequest demonstrates the premature return goroutine leak.
// This is one of the most common patterns found in production (see paper Section VII-A1).
func LeakyHandleRequest(item Item) error {
	ch := make(chan Result)

	go func() {
		result := expensiveComputation(item)
		ch <- result
	}()

	if item.Invalid {
		return errors.New("invalid item") // Early return - goroutine leaks!
	}

	// Normal path: receive the result
	result := <-ch
	_ = result
	return nil
}

func expensiveComputation(item Item) Result {
	// Simulate some expensive work
	return Result{Value: item.ID * 2}
}
