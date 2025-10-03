# Job Package

A flexible and robust job execution framework for Go that provides concurrent job management with retry, timeout, cooldown, and state tracking capabilities.

## Features

- **State Management**: Full lifecycle tracking (Created → Running → Succeeded/Failed/Canceled)
- **Progress Monitoring**: Real-time progress tracking (0.0 to 1.0)
- **Cancellation Support**: Graceful job cancellation with context awareness
- **Retry Logic**: Automatic retry with configurable attempts and delays
- **Timeout Management**: Per-execution timeout with context cancellation
- **Cooldown Periods**: Prevent rapid re-execution with configurable cooldowns
- **Once Constraint**: Ensure jobs run successfully only once (compatible with retries)
- **Result Storage**: Store and retrieve job results and parameters
- **Panic Recovery**: Automatic panic recovery and error propagation
- **Metadata Support**: Labels, timestamps, and execution statistics

## Architecture

The package consists of two main components:

- **Job** (`pkg/utils/job`): Core job abstraction with state management and lifecycle control
- **Executor** (`pkg/utils/job/executor`): Advanced execution policies (retry, timeout, cooldown, constraints)

## Quick Start

### Basic Job Execution

```go
import (
    "context"
    "github.com/xhanio/framingo/pkg/utils/job"
)

// Create and run a simple job
j := job.New("my-job", job.Wrap(func(ctx context.Context) error {
    // Your job logic here
    return nil
}))

j.Run(context.Background(), nil)
j.Wait()

if err := j.Err(); err != nil {
    log.Fatalf("Job failed: %v", err)
}
```

### Using Executor

```go
import (
    "time"
    "github.com/xhanio/framingo/pkg/utils/job/executor"
)

// Create job with execution policies
j := job.New("api-call", job.Wrap(callAPI))

exec := executor.New(j,
    executor.WithRetry(3, 2*time.Second),    // Retry 3 times
    executor.WithTimeout(30*time.Second),     // 30s timeout
    executor.WithCooldown(5*time.Minute),     // 5min cooldown
)

if err := exec.Start(context.Background(), nil); err != nil {
    log.Printf("Execution failed: %v", err)
}
```

### Progress Tracking

```go
j := job.New("upload", func(ctx job.Context) error {
    for i := 0; i < 100; i++ {
        ctx.SetProgress(float64(i) / 100.0)
        if err := uploadChunk(i); err != nil {
            return err
        }
    }
    return nil
})

// Monitor progress
go func() {
    for !j.IsDone() {
        fmt.Printf("Progress: %.0f%%\n", j.Progress()*100)
        time.Sleep(1 * time.Second)
    }
}()

j.Run(context.Background(), nil)
j.Wait()
```

## Job States

Jobs transition through the following states:

| State | Description |
|-------|-------------|
| `StateCreated` | Job initialized but not started |
| `StateRunning` | Job is currently executing |
| `StateCanceling` | Cancellation requested, job is shutting down |
| `StateSucceeded` | Job completed successfully |
| `StateFailed` | Job failed with an error |
| `StateCanceled` | Job was canceled before completion |

## Executor Options

| Option | Description | Example |
|--------|-------------|---------|
| `WithRetry(attempts, delay)` | Retry failed executions | `WithRetry(3, 2*time.Second)` |
| `WithTimeout(duration)` | Set execution timeout | `WithTimeout(30*time.Second)` |
| `WithCooldown(duration)` | Prevent rapid re-execution | `WithCooldown(5*time.Minute)` |
| `Once()` | Allow only one successful execution | `Once()` |
| `NoTimeout()` | Disable timeout | `NoTimeout()` |

### Combining Once() and WithRetry()

The `Once()` and `WithRetry()` options work together:

- **Once()**: Restricts the number of **successful** `Start()` calls to one
- **WithRetry()**: Allows retries **within a single `Start()` call**

```go
// Database migration: must succeed exactly once, but can retry on failures
exec := executor.New(migrationJob,
    executor.Once(),                      // Only one successful execution
    executor.WithRetry(5, 2*time.Second), // Retry transient failures
    executor.WithTimeout(30*time.Second), // Timeout per attempt
)

err := exec.Start(context.Background(), nil) // Retries up to 5 times
err = exec.Start(context.Background(), nil)  // Error: "job can only start once"
```

## Advanced Usage

### Job with Labels and Results

```go
j := job.New("fetch-data",
    func(ctx job.Context) error {
        data, err := fetchData()
        if err != nil {
            return err
        }
        ctx.SetResult(data)
        return nil
    },
    job.WithLabel("env", "production"),
    job.WithLabel("service", "api"),
)

j.Run(context.Background(), nil)
j.Wait()

// Retrieve result and labels
result := j.Result().(MyDataType)
labels := j.Labels()
```

### Job Cancellation

```go
j := job.New("long-running", job.Wrap(func(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Do work
            time.Sleep(1 * time.Second)
        }
    }
}))

// Run starts the job in a goroutine
j.Run(context.Background(), nil)

// Cancel after 5 seconds
time.Sleep(5 * time.Second)
j.Cancel()
j.Wait()

if j.IsState(job.StateCanceled) {
    fmt.Println("Job was canceled")
}
```

### Rate-Limited Execution

```go
exec := executor.New(scrapeJob,
    executor.WithCooldown(1*time.Hour),      // Max once per hour
    executor.WithRetry(3, 10*time.Second),
)

ticker := time.NewTicker(30 * time.Minute)
for range ticker.C {
    err := exec.Start(context.Background(), nil)
    if err != nil && strings.Contains(err.Error(), "cooldown") {
        log.Println("Skipping: still in cooldown")
    }
}
```

### Batch Processing

```go
batchJob := job.New("batch", func(ctx job.Context) error {
    items := ctx.GetParams().([]Item)
    total := len(items)

    for i, item := range items {
        select {
        case <-ctx.Context().Done():
            return ctx.Context().Err()
        default:
        }

        if err := processItem(item); err != nil {
            return err
        }

        ctx.SetProgress(float64(i+1) / float64(total))
    }
    return nil
})

batchJob.Run(context.Background(), items)
batchJob.Wait()
```

## Statistics and Monitoring

### Job Statistics

```go
stats := j.Stats()
fmt.Printf("Job: %s, State: %s, Duration: %v\n",
    stats.ID, stats.State, stats.ExecutionTime)
fmt.Printf("Progress: %.0f%%, Error: %s\n",
    stats.Progress*100, stats.Error)
```

### Executor Statistics

```go
stats := exec.Stats()
fmt.Printf("Retries: %d, Cooldown: %v\n",
    stats.Retries, stats.Cooldown)
```

## Reference

### Job Interface

```go
type Job interface {
    // Lifecycle
    Run(ctx context.Context, params any) bool
    Wait()
    Cancel() bool

    // State
    State() State
    IsState(state State) bool
    IsDone() bool
    IsExecuting() bool

    // Data
    ID() string
    Labels() labels.Set
    Result() any
    Err() error
    Context() context.Context
    Progress() float64

    // Timing
    CreatedAt() time.Time
    StartedAt() time.Time
    EndedAt() time.Time
    ExecutionTime() time.Duration

    // Statistics
    Stats() *Stats
}
```

### Job Context Interface

```go
type Context interface {
    ID() string
    Context() context.Context
    Logger() log.Logger
    Labels() labels.Set
    SetProgress(progress float64)
    SetResult(result any)
    GetParams() any
}
```

### Job Options

- `WithLabel(key, value string)`: Add single label
- `WithLabels(labels map[string]string)`: Add multiple labels
- `WithLogger(logger log.Logger)`: Set custom logger

### Executor Interface

```go
type Executor interface {
    Start(ctx context.Context, params any) error
    Stop(wait bool) error
    Stats() *Stats
}
```

### Executor Options

- `WithRetry(attempts int, delay time.Duration)`: Configure retry logic
- `WithTimeout(duration time.Duration)`: Set execution timeout
- `NoTimeout()`: Disable timeout
- `WithCooldown(duration time.Duration)`: Set cooldown period
- `Once()`: Allow only one successful execution

### Utility Functions

```go
job.IsDone(state State) bool   // Returns true for Succeeded/Failed/Canceled
job.IsPending(state State) bool // Returns true for Running/Canceling
```
