# httpretrier

[![Go Reference](https://pkg.go.dev/badge/github.com/slashdevops/httpretrier.svg)](https://pkg.go.dev/github.com/slashdevops/httpretrier)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/slashdevops/httpretrier?style=plastic)

`httpretrier` is a Go library that provides a **transparent** drop-in replacement for `http.Client` with automatic retry logic. It preserves all existing request headers (including authentication) while handling transient server errors (5xx) or network issues by retrying requests based on configurable strategies.

## Features

* **Transparent by Default:** Works as a zero-configuration drop-in replacement for `http.Client`, automatically preserving existing authentication tokens, custom headers, and all request properties without any code changes.
* **Automatic Retries:** Automatically retries requests that fail due to server errors (5xx) or transport-level errors.
* **Configurable Retry Strategies:**
  * `FixedDelay`: Retries after a constant delay.
  * `ExponentialBackoff`: Retries with exponentially increasing delays.
  * `JitterBackoff`: Retries with exponential backoff plus random jitter to prevent thundering herd issues.
* **Flexible Configuration:** Use the `ClientBuilder` for fine-grained control over:
  * Maximum number of retries.
  * Base and maximum delay for backoff strategies.
  * Standard `http.Transport` settings (timeouts, keep-alives, connection pooling).
  * Overall request timeout (`http.Client.Timeout`).
* **Easy Integration:** Designed as a complete drop-in replacement for `http.Client` - just change your client creation line and everything else works transparently.

## Installation

```bash
go get github.com/slashdevops/httpretrier@latest
```

## Update

```bash
go get -u github.com/slashdevops/httpretrier@latest
```

## Usage

### Basic Transparent Usage (Recommended)

The easiest way to add retry functionality is to replace your `http.Client` with `httpretrier.NewClient()`. It works transparently with all existing headers and authentication tokens.

```go
// Before: client := &http.Client{}
client := httpretrier.NewClient(3, httpretrier.ExponentialBackoff(500*time.Millisecond, 10*time.Second), nil)

// All your existing code works unchanged - auth tokens, custom headers, everything is preserved
req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
req.Header.Set("Authorization", "Bearer your-existing-token")
req.Header.Set("X-Custom-Header", "your-value")

resp, err := client.Do(req)
// Automatically retries on 5xx errors while preserving all headers
```

### Basic Usage with Specific Configuration

You can create a client with specific retry strategy and number of retries using `httpretrier.NewClient`.

```go
package main

import (
  "fmt"
  "io"
  "net/http"
  "net/http/httptest"
  "sync/atomic"
  "time"

  "github.com/slashdevops/httpretrier"
)

func main() {
  var requestCount int32
  // Example server that fails the first few requests
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    count := atomic.AddInt32(&requestCount, 1)
    if count <= 2 { // Fail first 2 times
      fmt.Printf("Server: Request %d -> 500 Internal Server Error\n", count)
      w.WriteHeader(http.StatusInternalServerError)
    } else {
      fmt.Printf("Server: Request %d -> 200 OK\n", count)
      w.WriteHeader(http.StatusOK)
      _, _ = w.Write([]byte("Success!"))
    }
  }))
  defer server.Close()

  // Create a client with exponential backoff (3 retries, 10ms base, 100ms max delay)
  retryClient := httpretrier.NewClient(
    3, // Max Retries
    httpretrier.ExponentialBackoff(10*time.Millisecond, 100*time.Millisecond),
    nil, // Use http.DefaultTransport
  )

  fmt.Println("Client: Making request...")
  resp, err := retryClient.Get(server.URL)
  if err != nil {
    fmt.Printf("Client: Request failed after retries: %v\n", err)
    return
  }
  defer resp.Body.Close()

  body, _ := io.ReadAll(resp.Body)
  fmt.Printf("Client: Received response: Status=%s, Body='%s'\n", resp.Status, string(body))
}

// Example Output:
// Client: Making request...
// Server: Request 1 -> 500 Internal Server Error
// Server: Request 2 -> 500 Internal Server Error
// Server: Request 3 -> 200 OK
// Client: Received response: Status=200 OK, Body='Success!'
```

### Using Custom Transport

You can provide your own `http.Transport` with specific settings for connection pooling, timeouts, TLS configuration, etc.:

```go
package main

import (
  "fmt"
  "net/http"
  "time"

  "github.com/slashdevops/httpretrier"
)

func main() {
  // Create a custom transport with specific settings
  customTransport := &http.Transport{
    MaxIdleConns:        50,                // Custom connection pool size
    IdleConnTimeout:     30 * time.Second, // Custom idle timeout
    DisableKeepAlives:   false,            // Enable keep-alives
    MaxIdleConnsPerHost: 10,               // Custom per-host connection limit
    TLSHandshakeTimeout: 5 * time.Second,  // Custom TLS timeout
  }

  // Create retry client with your custom transport
  client := httpretrier.NewClient(
    3, // Max retries
    httpretrier.ExponentialBackoff(100*time.Millisecond, 1*time.Second),
    customTransport, // Use your custom transport as the base
  )

  // Use the client normally - all your transport settings are preserved
  resp, err := client.Get("https://api.example.com/data")
  if err != nil {
    fmt.Printf("Request failed: %v\n", err)
    return
  }
  defer resp.Body.Close()

  // Your custom transport settings (connection pooling, timeouts) are used
  // while still getting automatic retry functionality
  fmt.Printf("Success with custom transport! Status: %d\n", resp.StatusCode)
}
```

### Advanced Configuration with ClientBuilder

For more control over the client and transport settings, use the `ClientBuilder`.

```go
package main

import (
  "fmt"
  "io"
  "net/http"
  "net/http/httptest"
  "time"

  "github.com/slashdevops/httpretrier"
)

func main() {
  // Example server
  server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("Builder success!"))
  }))
  defer server.Close()

  // Use the builder for detailed configuration
  builder := httpretrier.NewClientBuilder()

  httpClient := builder.
    WithTimeout(15 * time.Second).          // Overall request timeout
    WithMaxRetries(5).                      // Max 5 retries
    WithRetryStrategy(httpretrier.JitterBackoffStrategy). // Use Jitter strategy
    WithRetryBaseDelay(100 * time.Millisecond). // 100ms base delay
    WithRetryMaxDelay(2 * time.Second).       // 2s max delay
    WithMaxIdleConns(50).                   // Transport: Max 50 idle connections
    WithIdleConnTimeout(30 * time.Second).    // Transport: 30s idle timeout
    Build()                                 // Build the http.Client

  fmt.Println("Client (Builder): Making request...")
  resp, err := httpClient.Get(server.URL)
  if err != nil {
    fmt.Printf("Client (Builder): Request failed: %v\n", err)
    return
  }
  defer resp.Body.Close()

  body, _ := io.ReadAll(resp.Body)
  fmt.Printf("Client (Builder): Received response: Status=%s, Body='%s'\n", resp.Status, string(body))
}

// Example Output:
// Client (Builder): Making request...
// Client (Builder): Received response: Status=200 OK, Body='Builder success!'
```

## Configuration Options (ClientBuilder)

The `ClientBuilder` allows configuration of:

* **Retry Logic:**
  * `WithMaxRetries(int)`: Maximum number of retry attempts.
  * `WithRetryStrategy(httpretrier.Strategy)`: Set the strategy (`FixedDelayStrategy`, `ExponentialBackoffStrategy`, `JitterBackoffStrategy`).
  * `WithRetryBaseDelay(time.Duration)`: Base delay for backoff/jitter, or the fixed delay duration.
  * `WithRetryMaxDelay(time.Duration)`: Maximum delay cap for backoff/jitter strategies.
* **HTTP Client:**
  * `WithTimeout(time.Duration)`: Sets the `Timeout` field on the resulting `http.Client`.
* **HTTP Transport:** (Controls the underlying `http.Transport`)
  * `WithMaxIdleConns(int)`
  * `WithIdleConnTimeout(time.Duration)`
  * `WithTLSHandshakeTimeout(time.Duration)`
  * `WithExpectContinueTimeout(time.Duration)`
  * `WithDisableKeepAlives(bool)`
  * `WithMaxIdleConnsPerHost(int)`

See the Go documentation for default values and validation ranges for these parameters.

## License

This library is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
