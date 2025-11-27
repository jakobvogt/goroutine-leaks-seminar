package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jvogt/goroutine-leaks-seminar/examples/simple_deadlock"
)

func main() {
	example := flag.String("example", "simple", "which example to run: simple, send, circular")
	flag.Parse()

	switch *example {
	case "simple":
		fmt.Println("Running SimpleDeadlock: receiving from channel with no sender")
		simple_deadlock.SimpleDeadlock()
	case "send":
		fmt.Println("Running SendWithoutReceiver: sending on unbuffered channel with no receiver")
		simple_deadlock.SendWithoutReceiver()
	case "circular":
		fmt.Println("Running TwoGoroutineDeadlock: circular wait between goroutines")
		simple_deadlock.TwoGoroutineDeadlock()
	default:
		fmt.Fprintf(os.Stderr, "Unknown example: %s\n", *example)
		fmt.Fprintf(os.Stderr, "Available examples: simple, send, circular\n")
		os.Exit(1)
	}
}
