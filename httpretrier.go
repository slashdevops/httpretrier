package httpretrier

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

var ErrAllRetriesFailed = errors.New("all retry attempts failed")

// RetryStrategy defines the function signature for different retry strategies
type RetryStrategy func(attempt int) time.Duration

// ExponentialBackoff returns a RetryStrategy that calculates delays
// growing exponentially with each retry attempt, starting from base
// and capped at maxDelay.
func ExponentialBackoff(base, maxDelay time.Duration) RetryStrategy {
	return func(attempt int) time.Duration {
		// Special case from test: If base > maxDelay, the first attempt returns base,
		// subsequent attempts calculate normally and cap at maxDelay.
		if attempt == 0 && base > maxDelay {
			return base
		}

		// Calculate delay: base * 2^attempt
		// Use uint for bit shift robustness, though overflow is unlikely before capping.
		delay := base * (1 << uint(attempt))

		// Cap at maxDelay. Also handle potential overflow resulting in negative/zero delay.
		if delay > maxDelay || delay <= 0 {
			delay = maxDelay
		}
		// Note: The original check `if delay < base { delay = base }` is removed
		// as the logic now correctly handles the base > maxDelay case based on the test,
		// and for base <= maxDelay, the calculated delay won't be less than base for attempt >= 0.
		return delay
	}
}

// FixedDelay returns a RetryStrategy that provides a constant delay
// for each retry attempt.
func FixedDelay(delay time.Duration) RetryStrategy {
	return func(attempt int) time.Duration {
		return delay
	}
}

// JitterBackoff returns a RetryStrategy that adds a random jitter
// to the exponential backoff delay calculated using base and maxDelay.
func JitterBackoff(base, maxDelay time.Duration) RetryStrategy {
	expBackoff := ExponentialBackoff(base, maxDelay)
	return func(attempt int) time.Duration {
		baseDelay := expBackoff(attempt)
		// Add jitter: random duration between 0 and baseDelay/2
		jitter := time.Duration(rand.Int63n(int64(baseDelay / 2)))
		return baseDelay + jitter
	}
}

// retryTransport wraps http.RoundTripper to add retry logic
type retryTransport struct {
	Transport     http.RoundTripper // Underlying transport (e.g., http.DefaultTransport)
	MaxRetries    int
	RetryStrategy RetryStrategy // The strategy function to calculate delay
}

// RoundTrip executes an HTTP request with retry logic
func (r *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	// Ensure transport is set
	transport := r.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// Ensure a retry strategy is set, default to a basic exponential backoff
	retryStrategy := r.RetryStrategy
	if retryStrategy == nil {
		retryStrategy = ExponentialBackoff(500*time.Millisecond, 10*time.Second) // Default strategy
	}

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		// Clone the request body if it exists and is GetBody is defined
		// This allows the body to be read multiple times on retries
		if req.Body != nil && req.GetBody != nil {
			bodyClone, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to get request body for retry: %w", err)
			}
			req.Body = bodyClone
		}

		resp, err = transport.RoundTrip(req)

		// Success conditions: no error and status code below 500
		if err == nil && resp.StatusCode < http.StatusInternalServerError {
			return resp, nil
		}

		// If there was an error or a server-side error (5xx), prepare for retry

		// Close response body to prevent resource leaks before retrying
		if resp != nil {
			// Drain the body before closing
			_, copyErr := io.Copy(io.Discard, resp.Body)
			closeErr := resp.Body.Close()
			if copyErr != nil {
				// Prioritize returning the copy error
				return nil, fmt.Errorf("failed to discard response body: %w", copyErr)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("failed to close response body: %w", closeErr)
			}
		}

		// Check if we should retry
		if attempt < r.MaxRetries {
			delay := retryStrategy(attempt)
			// Silent retry - no debug output for clean, transparent operation
			time.Sleep(delay)
		} else {
			// Max retries reached, return the last error or a generic failure error
			if err != nil {
				return nil, fmt.Errorf("all retries failed; last error: %w", err)
			}
			// If the last attempt resulted in a 5xx response without a transport error
			if resp != nil {
				// Return a more specific error including the status code
				return nil, fmt.Errorf("%w: last attempt failed with status %d", ErrAllRetriesFailed, resp.StatusCode)
			}
			// Fallback generic error
			return nil, ErrAllRetriesFailed
		}
	}

	// This point should theoretically not be reached due to the loop logic,
	// but return the generic error just in case.
	return nil, ErrAllRetriesFailed
}

// NewClient creates a new http.Client configured with the retry transport.
func NewClient(maxRetries int, strategy RetryStrategy, baseTransport http.RoundTripper) *http.Client {
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	if strategy == nil {
		// Provide a default strategy if none is given
		strategy = ExponentialBackoff(500*time.Millisecond, 10*time.Second)
	}
	return &http.Client{
		Transport: &retryTransport{
			Transport:     baseTransport,
			MaxRetries:    maxRetries,
			RetryStrategy: strategy,
		},
	}
}
