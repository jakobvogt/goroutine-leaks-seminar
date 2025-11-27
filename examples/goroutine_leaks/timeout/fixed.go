package timeout

import (
	"context"
	"time"
)

// FixedFetchWithTimeout uses a buffered channel so send never blocks
func FixedFetchWithTimeout(ctx context.Context, item Item) (*Result, error) {
	ch := make(chan *Result, 1) // buffered!

	go func() {
		result := fetchDataFixed(item)
		ch <- result // never blocks now
	}()

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		// goroutine completes and sends to buffer, then gets GC'd
		return nil, ctx.Err()
	}
}

func fetchDataFixed(item Item) *Result {
	time.Sleep(50 * time.Millisecond)
	return &Result{Value: item.ID * 10}
}
