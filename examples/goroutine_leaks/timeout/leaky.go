package timeout

import (
	"context"
	"time"
)

type Item struct {
	ID int
}

type Result struct {
	Value int
}

// LeakyFetchWithTimeout demonstrates the "timeout leak" pattern.
// One of the most common patterns found by LeakProf in production (paper Section VII-A2).
//
// The problem: if ctx times out before fetchData completes, the goroutine
// blocks forever trying to send on ch (no receiver)
func LeakyFetchWithTimeout(ctx context.Context, item Item) (*Result, error) {
	ch := make(chan *Result) // unbuffered - this is the bug

	go func() {
		result := fetchData(item)
		ch <- result // blocks if ctx times out first
	}()

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err() // goroutine leaks here
	}
}

func fetchData(item Item) *Result {
	time.Sleep(50 * time.Millisecond) // simulate slow backend
	return &Result{Value: item.ID * 10}
}
