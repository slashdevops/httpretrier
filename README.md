# httpretrier

[![Go Reference](https://pkg.go.dev/badge/github.com/p2p-b2b/httpretrier.svg)](https://pkg.go.dev/github.com/p2p-b2b/httpretrier)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/p2p-b2b/httpretrier?style=plastic)

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
go get github.com/p2p-b2b/httpretrier
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

  "github.com/p2p-b2b/httpretrier"
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

  "github.com/p2p-b2b/httpretrier"
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

  "github.com/p2p-b2b/httpretrier"
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

## Authorization

The library provides built-in support for common HTTP authentication patterns. Authorization is applied to all requests, including retries, and automatically handles 401 Unauthorized responses by attempting to refresh credentials when supported.

### Bearer Token Authentication

```go
// Simple Bearer token
client := httpretrier.NewClientBuilder().
    WithBearerToken("your-access-token").
    WithMaxRetries(3).
    Build()

// Bearer token with automatic refresh
client := httpretrier.NewClientBuilder().
    WithBearerTokenAndRefresh("initial-token", func() (string, error) {
        // Your token refresh logic here
        return refreshTokenFromAPI()
    }).
    WithMaxRetries(3).
    Build()
```

### API Key Authentication

```go
// API key in custom header
client := httpretrier.NewClientBuilder().
    WithAPIKey("your-api-key", "X-API-Key").
    WithMaxRetries(3).
    Build()
```

### Basic Authentication

```go
client := httpretrier.NewClientBuilder().
    WithBasicAuth("username", "password").
    WithMaxRetries(3).
    Build()
```

### Custom Header Authentication

```go
// Multiple custom headers
headers := map[string]string{
    "X-Client-ID": "your-client-id",
    "X-Signature": "your-hmac-signature",
}

client := httpretrier.NewClientBuilder().
    WithCustomHeaders(headers).
    WithMaxRetries(3).
    Build()
```

### Custom Authorizer

For advanced authentication schemes, implement the `Authorizer` interface:

```go
type MyCustomAuth struct {
    // Your auth fields
}

func (a *MyCustomAuth) Authorize(req *http.Request) error {
    // Add your custom authorization logic
    req.Header.Set("Authorization", "Custom "+a.Token)
    return nil
}

func (a *MyCustomAuth) RefreshIfNeeded() error {
    // Optional: refresh logic for 401 responses
    return nil
}

// Use custom authorizer
client := httpretrier.NewClientBuilder().
    WithAuthorizer(&MyCustomAuth{}).
    WithMaxRetries(3).
    Build()
```

### Request-Level Authorization

For scenarios where Bearer tokens are already present in requests (like proxy servers, middleware, or request forwarding):

```go
// Use Bearer tokens from incoming requests
client := httpretrier.NewClientBuilder().
    WithRequestTokenAuth(false). // false = require token, true = allow empty
    WithMaxRetries(3).
    Build()

// With automatic token refresh on 401
client := httpretrier.NewClientBuilder().
    WithRequestTokenAuthAndRefresh(func(currentToken string) (string, error) {
        // Refresh the token based on current token
        return refreshToken(currentToken)
    }, false).
    WithMaxRetries(3).
    Build()

// Make request with existing Authorization header
req, _ := http.NewRequest("GET", "https://api.example.com", nil)
req.Header.Set("Authorization", "Bearer existing-token")
resp, err := client.Do(req)
```

### Passthrough Authorization

Preserve existing authorization headers and optionally provide fallback authentication:

```go
// Preserve existing auth, use fallback if none exists
client := httpretrier.NewClientBuilder().
    WithPassthroughAuth(NewBearerTokenAuth("fallback-token")).
    Build()
```

### Conditional Authorization

Apply different authentication strategies based on request context:

```go
client := httpretrier.NewClientBuilder().
    WithConditionalAuth(func(req *http.Request) Authorizer {
        service := req.Header.Get("X-Service")
        switch service {
        case "internal":
            return NewBearerTokenAuth("internal-token")
        case "external":
            return NewAPIKeyAuth("external-key", "X-API-Key")
        default:
            return nil // No auth
        }
    }).
    Build()
```

### Authorization with Retry Integration

Authorization works seamlessly with retry logic:

1. **Auth + Retry**: Authorization headers are added to each retry attempt
2. **401 Handling**: On 401 Unauthorized responses, the authorizer attempts to refresh credentials
3. **Automatic Retry**: After successful credential refresh, the request is automatically retried once
4. **Layered Approach**: Auth transport wraps retry transport, ensuring proper order of operations

## Configuration Options (ClientBuilder)

The `ClientBuilder` allows configuration of:

* **Authorization:**
  * `WithBearerToken(token string)`: Add Bearer token authentication.
  * `WithBearerTokenAndRefresh(token string, refreshFunc func() (string, error))`: Bearer token with refresh capability.
  * `WithAPIKey(key, header string)`: Add API key authentication in specified header.
  * `WithBasicAuth(username, password string)`: Add HTTP Basic authentication.
  * `WithCustomHeaders(headers map[string]string)`: Add custom header authentication.
  * `WithRequestTokenAuth(allowEmpty bool)`: Use Bearer tokens from incoming requests.
  * `WithRequestTokenAuthAndRefresh(refreshFunc func(string) (string, error), allowEmpty bool)`: Request-level tokens with refresh.
  * `WithPassthroughAuth(defaultAuth Authorizer)`: Preserve existing auth, use default if none.
  * `WithConditionalAuth(condition func(*http.Request) Authorizer)`: Context-aware authorization.
  * `WithAuthorizer(authorizer Authorizer)`: Use a custom authorizer implementation.
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
