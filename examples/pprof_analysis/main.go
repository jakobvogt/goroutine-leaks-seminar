// This example demonstrates how LeakProf detects goroutine leaks in production.
//
// It uses the same "timeout leak" pattern from examples/goroutine_leaks/timeout,
// showing how LeakProf would detect it in production when GoLeak misses it during testing.
//
// LeakProf uses Go's built-in pprof profiling to:
// 1. Capture goroutine profiles from running services
// 2. Identify goroutines blocked on channel operations
// 3. Flag locations with abnormally high concentrations of blocked goroutines
package main

import (
	"bytes"
	"context"
	"fmt"
	"runtime/pprof"
	"strings"
	"time"
)

func main() {
	fmt.Println("=== LeakProf Detection Demo ===")
	fmt.Println()
	fmt.Println("Simulating a production service with goroutine leaks...")
	fmt.Println()

	// Simulate the "timeout leak" pattern from production
	// This creates multiple leaked goroutines at the same location
	simulateTimeoutLeaks(5)

	// Give goroutines time to block
	time.Sleep(100 * time.Millisecond)

	// Capture and analyze goroutine profile (like LeakProf does)
	analyzeGoroutineProfile()
}

// simulateTimeoutLeaks creates N leaked goroutines using the timeout pattern
func simulateTimeoutLeaks(n int) {
	for i := 0; i < n; i++ {
		// Short timeout causes the leak
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		leakyFetch(ctx, i)
		cancel()
	}
}

// leakyFetch reproduces the problematic pattern (unbuffered channel + context timeout)
func leakyFetch(ctx context.Context, id int) {
	ch := make(chan int) // BUG: should be buffered

	go func() {
		time.Sleep(50 * time.Millisecond) // simulate slow work
		ch <- id * 10                     // blocks forever if ctx times out
	}()

	select {
	case result := <-ch:
		fmt.Printf("Got result: %d\n", result)
	case <-ctx.Done():
		// Context timed out -> goroutine is now leaked
		return
	}
}

// analyzeGoroutineProfile captures and analyzes goroutine profiles like LeakProf
// TODO: could probably clean this up a bit
func analyzeGoroutineProfile() {
	fmt.Println("=== Goroutine Profile Analysis (LeakProf approach) ===")
	fmt.Println()

	// Capture goroutine profile (what LeakProf fetches via HTTP from production)
	// debug=2 gives full stack traces with goroutine state
	var buf bytes.Buffer
	pprof.Lookup("goroutine").WriteTo(&buf, 2)

	profile := buf.String()

	// count goroutines by state
	lines := strings.Split(profile, "\n")

	blockedOnSend := 0
	blockedOnRecv := 0
	var leakLocations []string

	for i, line := range lines {
		if strings.Contains(line, "chan send") {
			blockedOnSend++
			// find the source location (next few lines contain it)
			// note: just looks for "pprof_analysis" to keep things simple
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				if strings.Contains(lines[j], "pprof_analysis") {
					leakLocations = append(leakLocations, strings.TrimSpace(lines[j]))
					break
				}
			}
		}
		if strings.Contains(line, "chan receive") {
			blockedOnRecv++
		}
	}

	fmt.Printf("Goroutines blocked on channel send: %d\n", blockedOnSend)
	fmt.Printf("Goroutines blocked on channel receive: %d\n", blockedOnRecv)
	fmt.Println()

	if blockedOnSend > 0 {
		fmt.Println("Potential leak locations (blocked on send):")
		// Deduplicate locations
		seen := make(map[string]int)
		for _, loc := range leakLocations {
			seen[loc]++
		}
		for loc, count := range seen {
			fmt.Printf("  [%d goroutines] %s\n", count, loc)
		}
		fmt.Println()
	}

	// LeakProf's key insight
	fmt.Println("=== LeakProf Detection Logic ===")
	fmt.Println()
	fmt.Println("LeakProf flags a location as suspicious when:")
	fmt.Printf("  - Many goroutines (>10K in production) blocked at SAME location\n")
	fmt.Println("  - The blocked goroutines accumulate over time")
	fmt.Println()
	fmt.Println("In this demo, we have", blockedOnSend, "goroutines blocked at the same location")
	fmt.Println()

	// Show the raw profile as well
	fmt.Println("=== Raw Goroutine Profile (truncated) ===")
	fmt.Println()
	// Only show first 2000 chars to keep output manageable
	if len(profile) > 2000 {
		fmt.Println(profile[:2000])
		fmt.Println("... (truncated)")
	} else {
		fmt.Println(profile)
	}
}
