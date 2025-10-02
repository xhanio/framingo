# Errors Package

A structured error handling package for Go applications that provides enhanced error management with categories, codes, stack traces, and error chaining.

## Features

- **Error Categories**: Predefined HTTP-compatible error categories (BadRequest, NotFound, Internal, etc.)
- **Error Codes**: Custom error codes with key-value details for structured error identification
- **Stack Traces**: Automatic stack trace capture for debugging
- **Error Chaining**: Support for wrapping and chaining errors with context
- **Flexible Formatting**: Multiple output formats including structured details and stack traces
- **Error Traversal**: Methods to navigate error chains and find specific causes

## Integration with Standard Errors

The package seamlessly integrates with Go's standard error handling:

```go
// Standard error
stdErr := fmt.Errorf("standard error")

// Wrap with enhanced error
enhanced := errors.Wrapf(stdErr, "enhanced context")

// Wrap returns nil if the parent error is nil
err := errors.Wrap(nil)  // err is nil
err := errors.Wrapf(nil, "some message")  // err is still nil

// Still works with standard error checking
if enhanced != nil {
    // Handle error
}
```

## Quick Start

### Creating Simple Errors

```go
// Simple error with message
err := errors.Newf("user not found")

// Category as error directly
err := errors.NotFound

// Using category shorthand
err := errors.NotFound.Newf("user %d not found", userID)
```

### Wrapping Errors

```go
// Direct wrap (preserves existing error, adds stack trace if not already an Error)
if err != nil {
    return errors.Wrap(err)
}

// Simple wrap with message
err := errors.Wrapf(originalErr, "processing failed for user %d", userID)

// Wrap with category
err := errors.Internal.Wrapf(dbErr, "database operation failed")
```

### Checking Errors

```go
// Check if error has specific category
if errors.Is(err, errors.NotFound) {
    // Handle not found error
}

// Check if error contains another error in chain
if errors.Has(err, originalErr) {
    // Handle case where originalErr is in the chain
}
```

## Error Categories

The package provides predefined categories that map to HTTP status codes:

| Category | HTTP Status | Description |
|----------|-------------|-------------|
| `BadRequest` | 400 | Invalid request parameters |
| `Unauthorized` | 401 | Authentication required |
| `Forbidden` | 403 | Permission denied |
| `NotFound` | 404 | Resource not found |
| `Conflict` | 409 | Resource conflict |
| `TooManyRequests` | 429 | Rate limit exceeded |
| `Internal` | 500 | Internal server error |
| `NotImplemented` | 501 | Feature not implemented |
| `Unavailable` | 503 | Service unavailable |

## Advanced Usage

### Creating Errors with Codes

```go
// Error with category and code
err := errors.BadRequest.New(
    errors.WithCode("INVALID_EMAIL", map[string]string{"field": "email"}),
    errors.WithMessage("invalid email: %s", email),
)

// Wrap with additional options
err := errors.Wrap(originalErr, errors.WithMessage("failed to process user"))
```

### Error Inspection

```go
// Create structured error
err := errors.BadRequest.New(
    errors.WithCode("VALIDATION_FAILED", map[string]string{
        "field": "email",
        "type": "format",
    }),
    errors.WithMessage("email validation failed: %s", email),
)

// Check error properties
code, details := err.(errors.Error).Code()
category := err.(errors.Error).Category()
message := err.(errors.Error).Message()

fmt.Printf("Code: %s, Status: %d\n", code, category.StatusCode())
```

### Error Chaining and Traversal

```go
// Create error chain
dbErr := errors.Internal.Newf("connection failed")
serviceErr := errors.Wrapf(dbErr, "user service error")
apiErr := errors.BadRequest.Wrapf(serviceErr, "API request failed")

// Navigate the chain
rootCause := apiErr.(errors.Error).RootCause()  // Gets dbErr
chain := apiErr.(errors.Error).Chain()          // Gets [apiErr, serviceErr, dbErr]

// Check for specific errors
if errors.Has(apiErr, dbErr) {
    // Handle database-related error
}
```

### Combining Multiple Errors

```go
var errs []error
errs = append(errs, errors.Newf("error 1"))
errs = append(errs, errors.Newf("error 2"))

combined := errors.Combine(errs...)
```

## Formatting and Output

### Stack Traces

Stack traces are automatically captured when creating new errors or wrapping non-Error types. Use `%+v` format to include stack traces in output:

```go
err := errors.Newf("something went wrong")
fmt.Printf("%+v", err) // Includes full stack trace
```

### Format Options

```go
err := errors.Internal.New(
    errors.WithCode("DB_ERROR", map[string]string{"table": "users"}),
    errors.WithMessage("database operation failed"),
)

// Different format options
fmt.Printf("%s", err)   // "database operation failed"
fmt.Printf("%m", err)   // "database operation failed" (message only)
fmt.Printf("%v", err)   // "{DB_ERROR:table=users} database operation failed\n[stack trace]"
```

## Reference

### Error Interface

Errors implement the `Error` interface providing these methods:

```go
type Error interface {
    Message() string              // Latest error message
    Error() string               // All messages concatenated
    Code() (string, labels.Set)  // Error code and details
    Category() Category          // Error category
    Has(cause error) bool        // Check if error exists in chain
    Cause() error               // Direct cause
    RootCause() error           // Root cause in chain
    Chain() []error             // All errors in chain
}
```

### Options

Customize errors using these options:

- `WithCode(code, details)`: Add error code and key-value details
- `WithMessage(format, args...)`: Set formatted error message
- `WithCategory(category)`: Set error category
