# Goroutine Leaks

This is a seminar project by Jakob Vogt, supervised by [Professor Martin Sulzmann](https://www.h-ka.de/die-hochschule-karlsruhe/organisation-personen/personen-a-z/person/martin-sulzmann) at Hochschule Karlsruhe - University of Applied Sciences.

---

## Introduction

### Background

This project is based on the paper ["Unveiling and Vanquishing Goroutine Leaks in Enterprise Microservices"](https://arxiv.org/pdf/2312.12002) by researchers from Aarhus University and Uber Technologies.

### What is a Goroutine Leak?

A goroutine leak (also called a "partial deadlock") occurs when a goroutine blocks on a channel operation and never finds a corresponding sender/receiver. The blocked goroutine:
- Remains in memory for the lifetime of the program
- Leaks its stack and any objects it references
- Cannot be garbage collected

Unlike traditional deadlocks where the entire program hangs, goroutine leaks are subtle and can accumulate over time, gradually degrading performance.

### What is GoLeak?

[GoLeak](https://github.com/uber-go/goleak) is a testing library from Uber that detects goroutine leaks by comparing goroutines before and after a test runs.
Any goroutine still present at test completion (excluding runtime background goroutines) is reported as a potential leak.

### What is LeakProf?

LeakProf is Uber's internal production monitoring tool that detects goroutine leaks in running services. While GoLeak catches leaks during testing, LeakProf finds leaks that escape to production due to:
- Complex control flows not covered by tests
- Timing-dependent bugs (for example timeouts under load)
- Unexplored thread interleavings

LeakProf works by periodically fetching goroutine profiles via Go's built-in pprof and flagging locations with abnormally high concentrations of blocked goroutines.

Note: Unlike [GoLeak](https://github.com/uber-go/goleak) which is open source, LeakProf is Uber's internal tool and is not publicly available. The examples in this project demonstrate the underlying technique LeakProf uses (analyzing pprof goroutine profiles) rather than running LeakProf itself.

### GoLeak vs LeakProf

The paper presents both tools as complementary: GoLeak **prevents** new leaks, LeakProf **detects** leaks that escape testing.

| Aspect | GoLeak | LeakProf |
|--------|--------|----------|
| **When** | Test time (CI/CD) | Production runtime |
| **How** | Snapshot goroutines at test end via `runtime.Stack` | Periodic goroutine profiles via pprof |
| **Purpose** | Prevention - block PRs with leaks | Detection - find leaks that escaped testing |
| **Precision** | 100% (no false positives) | 72.7% (threshold-based heuristics) |
| **Threshold** | Any lingering goroutine | >10K blocked goroutines at same location |
| **Results (paper)** | 857 found, ~260/year prevented | 24 found, 21 fixed |
| **Impact** | Stops leaks before merge | Up to 9.2× memory reduction, 34% CPU savings |


## Understanding Deadlocks vs Goroutine Leaks

### Deadlock Examples

A full deadlock occurs when all goroutines are blocked. Go's runtime detects this and crashes with:
```
fatal error: all goroutines are asleep - deadlock!
```

**Example 1: Receiving with no sender**
```go
func SimpleDeadlock() {
    ch := make(chan int)
    <-ch // Blocks forever
}
```

What happens:
- The main goroutine tries to receive from an unbuffered channel
- No other goroutine exists to send a value
- The main goroutine blocks forever
- Go runtime detects all goroutines are blocked and crashes

Example 2: Circular wait
```go
func TwoGoroutineDeadlock() {
    ch1 := make(chan int)
    ch2 := make(chan int)

    go func() {
        <-ch1      // Waits for ch1
        ch2 <- 1
    }()

    go func() {
        <-ch2      // Waits for ch2
        ch1 <- 1
    }()
    // Both goroutines wait for each other forever
}
```

**What happens:**
- First goroutine waits to receive from ch1
- Second goroutine waits to receive from ch2
- Neither can proceed because both are waiting for the other to send first
- Classic circular wait -> deadlock

### Key Difference: Deadlock vs Goroutine Leak

| Aspect | Deadlock | Goroutine Leak |
|--------|----------|----------------|
| Scope | ALL goroutines blocked | Only SOME goroutines blocked |
| Program behavior | Crashes immediately | Continues running |
| Detectability | Obvious (runtime crash) | Silent, requires tooling |
| Typical discovery | During development/testing | Often only in production |

### Why Go's Runtime Can't Detect Goroutine Leaks

Go's deadlock detector is pretty simplistic by design. It doesn't analyze the logic of your code or track relationships between goroutines.
Instead, it monitors the global execution state using two simple conditions:
1. **Are all goroutines blocked?** (Is the list of "Running" or "Runnable" goroutines empty?)
2. **Is there any "external hope"?** (Are there active timers, network listeners, or system calls?)

If all goroutines are blocked (1) AND nothing external can wake them (2), the runtime concludes the state can never change and crashes with a fatal deadlock error.

Why this fails to detect Goroutine Leaks:

With goroutine leaks, at least one goroutine keeps running (like a main loop or HTTP server in a microservice). Since not all goroutines are blocked, condition #1 is false, and the runtime takes no action.

But the problem runs deeper: even if the runtime *wanted* to detect partial deadlocks, it basically *can't*. The runtime doesn't track which goroutines hold references to which channels.
When a goroutine blocks on a channel receive, the runtime has no way to know whether another goroutine might eventually send on that channel (or whether the only goroutine that *could* send has already exited).
A goroutine legitimately waiting for an async operation looks identical to one that will never unblock.

This is why tools like GoLeak are necessary: the runtime's deadlock detection only catches total deadlocks, not partial ones.


## Goroutine Leak Examples

### Premature Return Leak

This is one of the more common leak patterns found in the paper (Section VII-A1).

#### The Problem (`leaky.go`)

```go
func LeakyHandleRequest(item Item) error {
    ch := make(chan Result)

    go func() {
        result := expensiveComputation(item)
        ch <- result  // Blocks here if no receiver!
    }()

    if item.Invalid {
        return errors.New("invalid")  // Early return -> goroutine leaks!
    }

    result := <-ch
    return nil
}
```

What happens:
- A goroutine is spawned to do expensive computation
- If validation fails, the parent function returns early
- The child goroutine tries to send on the channel, but no receiver exists
- Goroutine leaks

#### The Fix (`fixed.go`)

```go
func FixedHandleRequest(item Item) error {
    ch := make(chan Result, 1)  // Buffer size 1

    go func() {
        result := expensiveComputation(item)
        ch <- result  // Never blocks (buffer accepts the value)
    }()

    if item.Invalid {
        return errors.New("invalid")  // Safe to return early now
    }

    result := <-ch
    return nil
}
```

**Why this works:**
- A buffered channel can hold one value without a receiver
- The send completes immediately (value goes into buffer)
- The goroutine exits cleanly
- The channel and its value get garbage collected later

### Timeout Leak - Why GoLeak Isn't Enough

This example demonstrates **the limits of GoLeak** and why LeakProf is needed. The timeout leak is the most common pattern found by LeakProf in production (paper Section VII-A2).

#### The Problem (`leaky.go`)

```go
func LeakyFetchWithTimeout(ctx context.Context, item Item) (*Result, error) {
    ch := make(chan *Result)  // Unbuffered channel -> BUG!

    go func() {
        result := fetchData(item)
        ch <- result  // Blcks forever if ctx times out first
    }()

    select {
    case result := <-ch:
        return result, nil
    case <-ctx.Done():
        return nil, ctx.Err()  // Early return -> goroutine leaks!
    }
}
```

#### The Test (`leaky_test.go`) - GoLeak Passes!

```go
func TestLeakyFetchWithTimeout(t *testing.T) {
    defer goleak.VerifyNone(t)

    // Long timeout -> fetch completes before timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()

    result, err := LeakyFetchWithTimeout(ctx, Item{ID: 42})
    // ... assertions ...

    // GoLeak passes! The bug is still there, just not triggered.
}
```

This test passes, but the bug is still in the code. GoLeak doesn't detect it because:
- The test only exercises the happy path (fetch completes before timeout)
- The leak only occurs when the context times out
- Most developers tend to write tests like this and never trigger the timeout path

In production, under load:
- Slow backends, network issues, resource contention cause timeouts
- Each timeout leaks a goroutine blocked on `ch <- result`
- Thousands accumulate over time → memory grows → service degrades

**This is exactly why LeakProf exists:** to catch leaks that escaped testing by analyzing production goroutine profiles (see `examples/pprof_analysis`).

#### The Fix (`fixed.go`)

```go
func FixedFetchWithTimeout(ctx context.Context, item Item) (*Result, error) {
    ch := make(chan *Result, 1)  // Buffered channel - key fix!

    go func() {
        result := fetchData(item)
        ch <- result  // Never blocks (buffer accepts the value)
    }()

    select {
    case result := <-ch:
        return result, nil
    case <-ctx.Done():
        // Goroutine will complete and send to buffer
        // The channel and result get garbage collected
        return nil, ctx.Err()
    }
}
```

## Using GoLeak

GoLeak checks for unexpected goroutines at the end of a test:

### Test for the leaky version (`leaky_test.go`)

```go
func TestLeakyHandleRequest(t *testing.T) {
    defer goleak.VerifyNone(t)  // Checks for leaks when test ends

    // Call with invalid item -> triggers early return
    err := LeakyHandleRequest(Item{ID: 1, Invalid: true})
    if err == nil {
        t.Fatal("expected error for invalid item")
    }

    // GoLeak will detect the leaked goroutine and fail the test
}
```

### Test for the fixed version (`fixed_test.go`)

```go
func TestFixedHandleRequest(t *testing.T) {
    defer goleak.VerifyNone(t)

    err := FixedHandleRequest(Item{ID: 1, Invalid: true})
    if err == nil {
        t.Fatal("expected error for invalid item")
    }

    // GoLeak verifies no goroutines leaked
}
```

`defer goleak.VerifyNone(t)` runs after the test and fails if any unexpected goroutines remain.

## How GoLeak Detects Goroutine Leaks

GoLeak works on a simple principle:

> At the end of a test, there should be no lingering goroutines in the process address space.

Every partial deadlock violates this assumption. The converse isnt strictly true: a lingering goroutine might be waiting on a timer or IO rather than stuck on a channel.
However, such goroutines still indicate improper cleanup: the test didn't properly shut them down or wait for their completion.
Fixing these issues leads to better resource management and more reliable tests, so GoLeak treats all lingering goroutines (except known runtime internals) as problems worth fixing.


### The Detection Process

GoLeak uses the following approach to detect leaks:

1. **Test executes**: Your code runs normally
2. **Test ends**: GoLeak captures all goroutines via `runtime.Stack`
3. **Filter**: Removes the current goroutine and known system goroutines
4. **Retry**: Waits briefly and retries (goroutines may still be cleaning up)
5. **Report**: Any goroutines still present after filtering are reported as leaks

### What GoLeak Reports

For each leaked goroutine, GoLeak parses and stores:
- **ID**: The goroutine's unique identifier (e.g., `goroutine 7`)
- **State**: Current state like `chan send`, `chan receive`, `select`, `running`
- **First function**: The function at the top of the stack (where it's blocked)
- **Full stack**: The complete stack trace, including the `created by` line showing where the goroutine was spawned

Example output (from running `go test ./examples/goroutine_leaks/channel -run TestLeaky`):
```
found unexpected goroutines:
[Goroutine 7 in state chan send, with ...LeakyHandleRequest.func1 on top of the stack:
...LeakyHandleRequest.func1()
    .../leaky.go:23 +0x34
created by ...LeakyHandleRequest in goroutine 6
    .../leaky.go:21 +0x84
]
```

## How GoLeak Prevents Goroutine Leaks

GoLeak is most effective as a **preventive tool** when integrated into the development workflow.

### CI/CD Integration (from the paper)

The paper describes how Uber integrated GoLeak into their CI pipeline:

- Tests are instrumented with GoLeak (via `VerifyTestMain`)
- Every pull request runs the test suite
- If GoLeak detects a new leak, the PR is blocked
- Developers must fix the leak before merging

This catches leaks before they hit production.

### Filtering Known Goroutines

GoLeak provides options to ignore expected background goroutines:

```go
// Ignore a specific function at the top of the stack
goleak.VerifyNone(t, goleak.IgnoreTopFunction("mypackage.backgroundWorker"))

// Ignore a function anywhere in the stack
goleak.VerifyNone(t, goleak.IgnoreAnyFunction("mypackage.initLoop"))

// Ignore all goroutines that exist right now (useful in TestMain)
goleak.VerifyNone(t, goleak.IgnoreCurrent())
```

**Note:** The paper describes Uber's internal "suppression list" workflow for gradual adoption on legacy code. This is custom tooling built around GoLeak, not a built-in GoLeak feature.

### Prevention Statistics (from the paper)

At Uber's ~75 million line Go monorepo:
- **857** pre-existing goroutine leaks were found
- **~260** new leaks per year were prevented (estimated ~5 leaks/week before GoLeak)
- After deployment, new leaks dropped to **near zero**

## How GoLeak Works Internally

### Using Go's Runtime API

GoLeak leverages Go's built-in runtime capabilities. Internally, it uses the `runtime.Stack` function to collect stack traces of all goroutines:

```go
// Get stack traces of ALL goroutines in the process
buf := make([]byte, 1<<20)        // 1MB buffer
n := runtime.Stack(buf, true)     // true = include all goroutines
// buf[:n] now contains parseable stack trace text
```

You can see this in action by running:
```bash
go run ./examples/runtime_stack
```

### Information Extracted Per Goroutine

For each goroutine, GoLeak extracts:

| Field | Description | Example |
|-------|-------------|---------|
| **ID** | Unique goroutine identifier | `goroutine 7` |
| **State** | Current blocking state | `chan send`, `chan receive`, `select`, `running` |
| **Stack** | Full call stack | Function names + file:line locations |
| **Creator** | Where `go func()` was called | `main.go:15` |

### Filtering Background Goroutines

Not all lingering goroutines are leaks. GoLeak automatically filters out known system goroutines by matching their stack signatures:

| Filtered Goroutine | Why It's Ignored |
|--------------------|------------------|
| `testing.RunTests`, `testing.(*T).Run` | Go's test framework internals |
| `runtime.goexit` (in syscall) | CGo background operations |
| `os/signal.signal_recv`, `signal.loop` | Signal handling (Ctrl+C, etc.) |
| `runtime.ReadTrace` | Runtime tracing operations |

You can also add custom filters using `goleak.IgnoreTopFunction()` for application-specific background goroutines.

### Test Instrumentation Approaches

GoLeak offers two instrumentation methods:

1. Per-test verification:
```go
func TestSomething(t *testing.T) {
    defer goleak.VerifyNone(t)
    // test code
}
```

2. Package-wide verification via TestMain:
```go
func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

The `TestMain` approach is preferred for CI/CD as it:
- Automatically checks all tests in a package
- Can be injected by build tooling without modifying individual tests
- Runs after all tests complete, catching leaks from any test

## How LeakProf Works

While GoLeak catches leaks during testing, LeakProf finds leaks that escape to production. It's designed to detect leaks from complex contol flows, timing-dependent bugs, and missing test coverage.

### The Detection Process

LeakProf uses Go's built-in pprof profiling:

1. **Profile collection**: Periodically fetches goroutine profiles from production services via pprof HTTP endpoints
2. **Identify blocked goroutines**: Looks for goroutines with `runtime.gopark` at the top of the stack (indicates blocking)
3. **Categorize by operation**: Groups blocked goroutines by operation type (`chan send`, `chan receive`, `select`)
4. **Threshold detection**: Flags locations where >10K goroutines are blocked at the same source location
5. **Alert**: Reports suspicious locations to service owners with stack traces and memory impact

### Key Insight

> A few blocked goroutines is normal. Thousands blocked at the same location strongly suggests a leak.

This heuristic gives LeakProf 72.7% precision - not perfect, but the high impact bugs it finds (up to 9.2× memory reduction) justify the occasional false positive.

### How Blocked Goroutines Appear in Profiles

When LeakProf analyzes a goroutine profile, blocked goroutines have a distinct signature:

```
goroutine 4 [chan send]:           <- State indicates blocking
main.leakyFetch.func1()
    /path/to/leaky.go:56 +0x50     <- Source location of the block
created by main.leakyFetch in goroutine 1
    /path/to/leaky.go:53 +0x84     <- Where the goroutine was spawned
```

LeakProf counts how many goroutines are blocked at each source location. If the same line (e.g., `leaky.go:56`) appears in thousnads of goroutines, it's flagged as a potential leak.

### Production Impact (from the paper)

LeakProf found 24 leaks in production, of which 21 were fixed:
- One fix reduced memory by **9.2×** (from 6 GiB to 650 MiB)
- Another reduced CPU utilization by **34%**
- Several services had been unknowingly over-provisioned to compensate for leak-induced memory growth

## Running the Examples

### Overview

| Example | Type | Location |
|---------|------|----------|
| Simple Deadlock | Deadlock | `examples/simple_deadlock/` |
| Premature Return | Goroutine Leak (GoLeak detects) | `examples/goroutine_leaks/channel/` |
| Timeout Leak | GoLeak's Limits (test passes, bug remains) | `examples/goroutine_leaks/timeout/` |
| Runtime Stack API | GoLeak Internals | `examples/runtime_stack/` |
| LeakProf Technique | pprof Analysis (how LeakProf works) | `examples/pprof_analysis/` |

### Deadlock Examples

Deadlock examples crash the runtime, so they run as standalone programs:

```bash
# Receiving with no sender
go run ./examples/simple_deadlock/cmd -example=simple

# Sending with no receiver
go run ./examples/simple_deadlock/cmd -example=send

# Circular wait between goroutines
go run ./examples/simple_deadlock/cmd -example=circular
```

Each will output:
```
fatal error: all goroutines are asleep - deadlock!
```

### Goroutine Leak Examples (GoLeak)

Leak examples use GoLeak and run as tests:

Premature Return Leak:
```bash
# See GoLeak detect the leak (test fails)
go test ./examples/goroutine_leaks/channel -run TestLeaky -v

# See the fix work (test passes)
go test ./examples/goroutine_leaks/channel -run TestFixed -v
```

Timeout Leak (demonstrates GoLeak's limits):
```bash
# This test PASSES -> GoLeak doesn't catch the bug!
# The bug exists, but the test only exercises the happy path
go test ./examples/goroutine_leaks/timeout -run TestLeakyFetchWithTimeout -v

# The fix also passes
go test ./examples/goroutine_leaks/timeout -run TestFixed -v
```

Both tests pass, but only the fixed version is actually safe. The leaky version has a bug that only shows up when timeouts occur in production.

### GoLeak Internals Example

See how Go's `runtime.Stack` API works (the foundation of GoLeak):

```bash
go run ./examples/runtime_stack
```

This will show the raw stack trace output that GoLeak parses to detect leaks.

### LeakProf Technique Demo

This demo shows the **technique** LeakProf uses (since LeakProf itself is not publicly available).
It uses the same timeout leak pattern from `examples/goroutine_leaks/timeout` and demonstrates how LeakProf would detect it in production when GoLeak misses it during testing.

```bash
go run ./examples/pprof_analysis
```

The demo:
1. Creates multiple leaked goroutines using the same timeout pattern as `goroutine_leaks/timeout`
2. Captures a goroutine profile via pprof (like LeakProf does in production)
3. Analyzes blocked goroutines and identifies the leak location
4. Shows the raw profile data that LeakProf would process

Example output:
```
=== Goroutine Profile Analysis (LeakProf approach) ===

Goroutines blocked on channel send: 5
Potential leak locations (blocked on send):
  [5 goroutines] /path/to/pprof_analysis/main.go:56

=== LeakProf Detection Logic ===
LeakProf flags a location as suspicious when:
  - Many goroutines (>10K in production) blocked at SAME location
```
