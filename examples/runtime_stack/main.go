// Demonstrates how Go's runtime.Stack API works, which is basically
// how GoLeak detects goroutine leaks under the hood
package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	fmt.Println("=== Demonstrating runtime.Stack ===")
	fmt.Println()

	// start a goroutine that blocks on channel recieve (simulating a leak)
	go func() {
		ch := make(chan int)
		<-ch // blocks forever, no sender
	}()

	// another one that blocks on send
	go func() {
		ch := make(chan int)
		ch <- 42 // blocks forever
	}()

	time.Sleep(10 * time.Millisecond) // give goroutines time to block

	// Get stack traces of ALL goroutines in the process.
	// This is basically how GoLeak works
	buf := make([]byte, 1<<20)    // 1MB buffer
	n := runtime.Stack(buf, true) // true = all goroutines

	fmt.Printf("Captured %d bytes of stack trace data:\n", n)
	fmt.Println("-------------------------------------------")
	fmt.Println(string(buf[:n]))
	fmt.Println("-------------------------------------------")
	fmt.Println()
	fmt.Println("Each goroutine shows:")
	fmt.Println("  - Goroutine ID (e.g. 'goroutine 5')")
	fmt.Println("  - State ('[chan receive]', '[chan send]', etc)")
	fmt.Println("  - Stack trace with function names and file:line")
	fmt.Println("  - Where it was created ('created by ...')")
	fmt.Println()
	fmt.Println("GoLeak parses this to find lingering goroutines.")
}
