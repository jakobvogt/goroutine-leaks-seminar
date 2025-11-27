package channel

import "errors"

// FixedHandleRequest demonstrates the proper fix for the premature return leak.
func FixedHandleRequest(item Item) error {
	// Key fix: buffered channel with size 1
	ch := make(chan Result, 1)

	go func() {
		result := expensiveComputation(item)
		ch <- result // This send will NOT block because channel has buffer
		// Goroutine can now exit cleanly even if no one receives
	}()

	if item.Invalid {
		return errors.New("invalid item") // Early return is now safe!
	}

	// Normal path: receive the result
	result := <-ch
	_ = result
	return nil
}
