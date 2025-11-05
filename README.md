# Goroutine Leaks

This is a seminar project by Jakob Vogt, supervised by Professor Martin Sulzmann at Hochschule Karlsruhe - University of Applied Sciences.

---

## Introduction

### Background

This project is based on the paper ["Unveiling and Vanquishing Goroutine Leaks in Enterprise Microservices"](https://arxiv.org/pdf/2312.12002) by researchers from Aarhus University and Uber Technologies.

### What is a Goroutine Leak?

A goroutine leak (also called a "partial deadlock") occurs when a goroutine blocks on a channel operation and never finds a corresponding sender/receiver, causing it to:
- Remain in memory forever
- Leak its stack and any objects reachable from it
- Cannot be garbage collected

Unlike traditional deadlocks where the entire program hangs, goroutine leaks are subtle and can accumulate over time, gradually degrading performance.

### What is GoLeak?

[GoLeak](https://github.com/uber-go/goleak) is a testing library from Uber that helps identify goroutine leaks in Go tests by comparing goroutines before and after a test runs.


## Goroutine Leak Examples

### The Example: Premature Return Leak

This example shows the **most common goroutine leak pattern found in production** (see paper Section VII-A1: "Premature Function Return Leak").

#### The Problem (`leaky.go`)

```go
func LeakyHandleRequest(item Item) error {
    ch := make(chan Result)

    go func() {
        result := expensiveComputation(item)
        ch <- result  // Blocks here if no receiver!
    }()

    if item.Invalid {
        return errors.New("invalid")  // Early return - goroutine leaks!
    }

    result := <-ch
    return nil
}
```

**What happens:**
1. A goroutine is spawned to do expensive computation
2. Parent function does validation and returns early on error
3. Child goroutine tries to send on the channel but blocks forever (no receiver)
4. Goroutine leaks memory and never exits

#### The Fix (`fixed.go`)

The simplest solution: **use a buffered channel with size 1**

```go
func FixedHandleRequest(item Item) error {
    ch := make(chan Result, 1)  // Buffer size 1

    go func() {
        result := expensiveComputation(item)
        ch <- result  // Never blocks (buffer holds the value)
    }()

    if item.Invalid {
        return errors.New("invalid")  // Safe to return early now
    }

    result := <-ch
    return nil
}
```

## Running the Demo

### See GoLeak detect the leak (test fails):
```bash
go test ./examples/channel -run TestLeaky -v
```

You'll see GoLeak fail the test with a message like:
```
found unexpected goroutines:
[Goroutine 5 in state chan send, with channel.LeakyHandleRequest.func1...]
```

### See the fix work (test passes):
```bash
go test ./examples/channel -run TestFixed -v
```

### Run all tests:
```bash
go test ./...
```
