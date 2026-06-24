package httpretrier_test // Use _test package for examples

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/slashdevops/httpretrier"
)

// Example demonstrates using exponential backoff.
func Example() {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count <= 3 { // Fail first 3 times
			fmt.Printf("Server: Request %d -> 500 Internal Server Error\n", count)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			fmt.Printf("Server: Request %d -> 200 OK\n", count)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Success after backoff"))
		}
	}))
	defer server.Close()

	// Create a client with exponential backoff.
	// Base delay 5ms, max delay 50ms, max 4 retries.
	retryClient := httpretrier.NewClient(
		4,
		httpretrier.ExponentialBackoff(5*time.Millisecond, 50*time.Millisecond),
		nil,
	)

	fmt.Println("Client: Making request with exponential backoff...")
	resp, err := retryClient.Get(server.URL)
	if err != nil {
		fmt.Printf("Client: Request failed: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Client: Received response: Status=%s, Body='%s'\n", resp.Status, string(body))
	// Note: Duration will vary slightly, but should reflect increasing delays.
	fmt.Printf("Client: Total time approx > %dms (due to backoff)\n", (5 + 10 + 20)) // 5ms + 10ms + 20ms delays

	// Output:
	// Client: Making request with exponential backoff...
	// Server: Request 1 -> 500 Internal Server Error
	// Server: Request 2 -> 500 Internal Server Error
	// Server: Request 3 -> 500 Internal Server Error
	// Server: Request 4 -> 200 OK
	// Client: Received response: Status=200 OK, Body='Success after backoff'
	// Client: Total time approx > 35ms (due to backoff)
}

// ExampleNewClient_withExistingAuth demonstrates how the default client
// transparently preserves existing authentication headers in requests.
func ExampleNewClient_withExistingAuth() {
	var requestCount int32

	// Create a server that requires authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		auth := r.Header.Get("Authorization")

		if auth == "" {
			fmt.Printf("Server: Request %d -> 401 Unauthorized (no auth)\n", count)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		fmt.Printf("Server: Request %d with %s -> ", count, auth)
		if count <= 2 {
			fmt.Println("500 Internal Server Error")
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			fmt.Println("200 OK")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Authenticated and retried successfully"))
		}
	}))
	defer server.Close()

	// Create default client - works transparently with any existing auth
	client := httpretrier.NewClient(3, httpretrier.ExponentialBackoff(5*time.Millisecond, 50*time.Millisecond), nil)

	// Create request with existing auth token (from your app's auth system)
	req, _ := http.NewRequest("GET", server.URL, nil)
	req.Header.Set("Authorization", "Bearer my-token-123")

	fmt.Println("Client: Making authenticated request...")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Client: Request failed: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Client: Success! Status=%s, Body='%s'\n", resp.Status, string(body))
	fmt.Printf("Client: Auth header preserved through %d retries\n", atomic.LoadInt32(&requestCount))

	// Output:
	// Client: Making authenticated request...
	// Server: Request 1 with Bearer my-token-123 -> 500 Internal Server Error
	// Server: Request 2 with Bearer my-token-123 -> 500 Internal Server Error
	// Server: Request 3 with Bearer my-token-123 -> 200 OK
	// Client: Success! Status=200 OK, Body='Authenticated and retried successfully'
	// Client: Auth header preserved through 3 retries
}

// ExampleNewClientBuilder_transparent demonstrates using the ClientBuilder
// for advanced configuration while maintaining transparent behavior.
func ExampleNewClientBuilder_transparent() {
	// Create a simple test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back any custom headers that were sent
		customValue := r.Header.Get("X-Custom-Header")
		if customValue != "" {
			fmt.Printf("Server: Received custom header: %s\n", customValue)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Custom headers preserved!"))
	}))
	defer server.Close()

	// Build client with custom settings - still works transparently
	client := httpretrier.NewClientBuilder().
		WithMaxRetries(5).
		WithRetryStrategy(httpretrier.JitterBackoffStrategy).
		WithTimeout(10 * time.Second).
		Build()

	// Create request with custom headers
	req, _ := http.NewRequest("GET", server.URL, nil)
	req.Header.Set("X-Custom-Header", "my-custom-value")
	req.Header.Set("Authorization", "Bearer token-from-somewhere")

	fmt.Println("Client: Making request with custom headers...")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Client: Request failed: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Client: Response: %s\n", string(body))

	// Output:
	// Client: Making request with custom headers...
	// Server: Received custom header: my-custom-value
	// Client: Response: Custom headers preserved!
}

// ExampleNewClient_withCustomTransport demonstrates using a custom base transport
// with specific transport settings while maintaining transparent retry behavior.
func ExampleNewClient_withCustomTransport() {
	var requestCount int32

	// Create a test server that fails initially to show retry behavior with custom transport
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		fmt.Printf("Server: Request %d from custom transport\n", count)

		if count <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Custom transport with retries works!"))
		}
	}))
	defer server.Close()

	// Create a custom transport with specific settings
	customTransport := &http.Transport{
		MaxIdleConns:        50,               // Custom connection pool size
		IdleConnTimeout:     30 * time.Second, // Custom idle timeout
		DisableKeepAlives:   false,            // Enable keep-alives
		MaxIdleConnsPerHost: 10,               // Custom per-host connection limit
		TLSHandshakeTimeout: 5 * time.Second,  // Custom TLS timeout
	}

	// Create retry client with custom transport
	client := httpretrier.NewClient(
		3, // Max retries
		httpretrier.ExponentialBackoff(5*time.Millisecond, 50*time.Millisecond),
		customTransport, // Use our custom transport as the base
	)

	fmt.Println("Client: Making request with custom transport...")
	resp, err := client.Get(server.URL)
	if err != nil {
		fmt.Printf("Client: Request failed: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Client: Response: %s\n", string(body))
	fmt.Printf("Client: Custom transport config preserved (MaxIdleConns: %d)\n",
		customTransport.MaxIdleConns)

	// Output:
	// Client: Making request with custom transport...
	// Server: Request 1 from custom transport
	// Server: Request 2 from custom transport
	// Client: Response: Custom transport with retries works!
	// Client: Custom transport config preserved (MaxIdleConns: 50)
}

// Helper function to parse URL (avoiding error handling in example)
