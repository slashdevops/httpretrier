package httpretrier

import (
	"log/slog"
	"net/http"
	"time"
)

const (
	ValidMaxIdleConns             = 200
	ValidMinIdleConns             = 1
	ValidMaxIdleConnsPerHost      = 200
	ValidMinIdleConnsPerHost      = 1
	ValidMaxIdleConnTimeout       = 120 * time.Second
	ValidMinIdleConnTimeout       = 1 * time.Second
	ValidMaxTLSHandshakeTimeout   = 15 * time.Second
	ValidMinTLSHandshakeTimeout   = 1 * time.Second
	ValidMaxExpectContinueTimeout = 5 * time.Second
	ValidMinExpectContinueTimeout = 1 * time.Second
	ValidMaxTimeout               = 30 * time.Second
	ValidMinTimeout               = 1 * time.Second
	ValidMaxRetries               = 10
	ValidMinRetries               = 1
	ValidMaxBaseDelay             = 5 * time.Second
	ValidMinBaseDelay             = 300 * time.Millisecond
	ValidMaxMaxDelay              = 120 * time.Second
	ValidMinMaxDelay              = 300 * time.Millisecond

	// DefaultMaxRetries is the default number of retry attempts
	DefaultMaxRetries = 3

	// DefaultBaseDelay is the default base delay for backoff strategies
	DefaultBaseDelay = 500 * time.Millisecond

	// DefaultMaxDelay is the default maximum delay for backoff strategies
	DefaultMaxDelay = 10 * time.Second

	// DefaultMaxIdleConns is the default maximum number of idle connections
	DefaultMaxIdleConns = 100

	// DefaultIdleConnTimeout is the default idle connection timeout
	DefaultIdleConnTimeout = 90 * time.Second

	// DefaultTLSHandshakeTimeout is the default TLS handshake timeout
	DefaultTLSHandshakeTimeout = 10 * time.Second

	// DefaultExpectContinueTimeout is the default expect continue timeout
	DefaultExpectContinueTimeout = 1 * time.Second

	// DefaultDisableKeepAlives is the default disable keep-alives setting
	DefaultDisableKeepAlives = false

	// DefaultMaxIdleConnsPerHost is the default maximum number of idle connections per host
	DefaultMaxIdleConnsPerHost = 100

	// DefaultTimeout is the default timeout for HTTP requests
	DefaultTimeout = 5 * time.Second
)

// ClientError represents an error that occurs during HTTP client operations
type ClientError struct {
	Message string
}

func (e *ClientError) Error() string {
	return e.Message
}

// Strategy defines the type for retry strategies
// It is a string type to allow for easy conversion from string literals
// to the defined types
type Strategy string

const (
	FixedDelayStrategy         Strategy = "fixed"
	JitterBackoffStrategy      Strategy = "jitter"
	ExponentialBackoffStrategy Strategy = "exponential"
)

func (s Strategy) String() string {
	return string(s)
}

func (s Strategy) IsValid() bool {
	switch s {
	case FixedDelayStrategy, JitterBackoffStrategy, ExponentialBackoffStrategy:
		return true
	default:
		return false
	}
}

// Client is a custom HTTP client with configurable settings
// and retry strategies. Works transparently with existing request headers.
type Client struct {
	maxIdleConns          int
	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	expectContinueTimeout time.Duration
	disableKeepAlives     bool
	maxIdleConnsPerHost   int
	timeout               time.Duration
	maxRetries            int
	retryStrategyType     Strategy // Store the type, not the function
	retryBaseDelay        time.Duration
	retryMaxDelay         time.Duration
}

// ClientBuilder is a builder for creating a custom HTTP client
type ClientBuilder struct {
	client *Client
}

// NewClientBuilder creates a new ClientBuilder with default settings
// and retry strategy
func NewClientBuilder() *ClientBuilder {
	cb := &ClientBuilder{
		client: &Client{
			// Initialize with defaults
			maxIdleConns:          DefaultMaxIdleConns,
			idleConnTimeout:       DefaultIdleConnTimeout,
			tlsHandshakeTimeout:   DefaultTLSHandshakeTimeout,
			expectContinueTimeout: DefaultExpectContinueTimeout,
			disableKeepAlives:     DefaultDisableKeepAlives,
			maxIdleConnsPerHost:   DefaultMaxIdleConnsPerHost,
			timeout:               DefaultTimeout,
			maxRetries:            DefaultMaxRetries,
			retryStrategyType:     ExponentialBackoffStrategy, // Default strategy type
			retryBaseDelay:        DefaultBaseDelay,
			retryMaxDelay:         DefaultMaxDelay,
		},
	}
	return cb
}

// WithMaxIdleConns sets the maximum number of idle connections
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithMaxIdleConns(maxIdleConns int) *ClientBuilder {
	// Just set the value, Build will validate/default
	b.client.maxIdleConns = maxIdleConns
	return b
}

// WithIdleConnTimeout sets the idle connection timeout
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithIdleConnTimeout(idleConnTimeout time.Duration) *ClientBuilder {
	b.client.idleConnTimeout = idleConnTimeout
	return b
}

// WithTLSHandshakeTimeout sets the TLS handshake timeout
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithTLSHandshakeTimeout(tlsHandshakeTimeout time.Duration) *ClientBuilder {
	b.client.tlsHandshakeTimeout = tlsHandshakeTimeout
	return b
}

// WithExpectContinueTimeout sets the expect continue timeout
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithExpectContinueTimeout(expectContinueTimeout time.Duration) *ClientBuilder {
	b.client.expectContinueTimeout = expectContinueTimeout
	return b
}

// WithDisableKeepAlives sets whether to disable keep-alives
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithDisableKeepAlives(disableKeepAlives bool) *ClientBuilder {
	b.client.disableKeepAlives = disableKeepAlives
	return b
}

// WithMaxIdleConnsPerHost sets the maximum number of idle connections per host
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithMaxIdleConnsPerHost(maxIdleConnsPerHost int) *ClientBuilder {
	b.client.maxIdleConnsPerHost = maxIdleConnsPerHost
	return b
}

// WithTimeout sets the timeout for HTTP requests
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithTimeout(timeout time.Duration) *ClientBuilder {
	b.client.timeout = timeout
	return b
}

// WithMaxRetries sets the maximum number of retry attempts
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithMaxRetries(maxRetries int) *ClientBuilder {
	b.client.maxRetries = maxRetries
	return b
}

// WithRetryBaseDelay sets the base delay for retry strategies
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithRetryBaseDelay(baseDelay time.Duration) *ClientBuilder {
	b.client.retryBaseDelay = baseDelay
	return b
}

// WithRetryMaxDelay sets the maximum delay for retry strategies
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithRetryMaxDelay(maxDelay time.Duration) *ClientBuilder {
	b.client.retryMaxDelay = maxDelay
	return b
}

// WithRetryStrategy sets the retry strategy type
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithRetryStrategy(strategy Strategy) *ClientBuilder {
	// Set the value as-is, validation happens during Build()
	b.client.retryStrategyType = strategy
	return b
}

// WithRetryStrategyAsString sets the retry strategy type from a string
// and returns the ClientBuilder for method chaining
func (b *ClientBuilder) WithRetryStrategyAsString(strategy string) *ClientBuilder {
	s := Strategy(strategy)
	if !s.IsValid() {
		slog.Warn("Invalid retry strategy type, using default (Exponential)", "invalidValue", s, "defaultValue", ExponentialBackoffStrategy)
		s = ExponentialBackoffStrategy
	}
	b.client.retryStrategyType = s
	return b
}

// Build creates and returns a new HTTP client with the specified settings
// and retry strategy. The client works transparently, preserving any existing
// headers in requests without requiring explicit configuration.
func (b *ClientBuilder) Build() *http.Client {
	// validate the settings and set defaults if necessary

	if b.client.maxIdleConns < ValidMinIdleConns || b.client.maxIdleConns > ValidMaxIdleConns {
		slog.Warn("Invalid max idle connections, using default value", "invalidValue", b.client.maxIdleConns, "defaultValue", DefaultMaxIdleConns)
		b.client.maxIdleConns = DefaultMaxIdleConns
	}

	if b.client.idleConnTimeout < ValidMinIdleConnTimeout || b.client.idleConnTimeout > ValidMaxIdleConnTimeout {
		slog.Warn("Invalid idle connection timeout, using default value", "invalidValue", b.client.idleConnTimeout, "defaultValue", DefaultIdleConnTimeout)
		b.client.idleConnTimeout = DefaultIdleConnTimeout
	}

	if b.client.tlsHandshakeTimeout < ValidMinTLSHandshakeTimeout || b.client.tlsHandshakeTimeout > ValidMaxTLSHandshakeTimeout {
		slog.Warn("Invalid TLS handshake timeout, using default value", "invalidValue", b.client.tlsHandshakeTimeout, "defaultValue", DefaultTLSHandshakeTimeout)
		b.client.tlsHandshakeTimeout = DefaultTLSHandshakeTimeout
	}

	if b.client.expectContinueTimeout < ValidMinExpectContinueTimeout || b.client.expectContinueTimeout > ValidMaxExpectContinueTimeout {
		slog.Warn("Invalid expect continue timeout, using default value", "invalidValue", b.client.expectContinueTimeout, "defaultValue", DefaultExpectContinueTimeout)
		b.client.expectContinueTimeout = DefaultExpectContinueTimeout
	}

	if b.client.maxIdleConnsPerHost < ValidMinIdleConnsPerHost || b.client.maxIdleConnsPerHost > ValidMaxIdleConnsPerHost {
		slog.Warn("Invalid max idle connections per host, using default value", "invalidValue", b.client.maxIdleConnsPerHost, "defaultValue", DefaultMaxIdleConnsPerHost)
		b.client.maxIdleConnsPerHost = DefaultMaxIdleConnsPerHost
	}

	if b.client.timeout < ValidMinTimeout || b.client.timeout > ValidMaxTimeout {
		slog.Warn("Invalid timeout, using default value", "invalidValue", b.client.timeout, "defaultValue", DefaultTimeout)
		b.client.timeout = DefaultTimeout
	}

	if b.client.maxRetries < ValidMinRetries || b.client.maxRetries > ValidMaxRetries {
		slog.Warn("Invalid max retries, using default value", "invalidValue", b.client.maxRetries, "defaultValue", DefaultMaxRetries)
		b.client.maxRetries = DefaultMaxRetries
	}

	if b.client.retryBaseDelay < ValidMinBaseDelay || b.client.retryBaseDelay > ValidMaxBaseDelay {
		slog.Warn("Invalid retry base delay, using default value", "invalidValue", b.client.retryBaseDelay, "defaultValue", DefaultBaseDelay)
		b.client.retryBaseDelay = DefaultBaseDelay
	}

	if b.client.retryMaxDelay < ValidMinMaxDelay || b.client.retryMaxDelay > ValidMaxMaxDelay {
		slog.Warn("Invalid retry max delay, using default value", "invalidValue", b.client.retryMaxDelay, "defaultValue", DefaultMaxDelay)
		b.client.retryMaxDelay = DefaultMaxDelay
	}

	// Determine the final strategy type, defaulting if necessary
	finalStrategyType := b.client.retryStrategyType
	switch finalStrategyType {
	case FixedDelayStrategy, JitterBackoffStrategy, ExponentialBackoffStrategy:
		// Valid type provided
	default:
		// No type set or invalid type somehow persisted, use default
		slog.Warn("No valid retry strategy type set, using default (Exponential)", "currentType", finalStrategyType)
		finalStrategyType = ExponentialBackoffStrategy
	}

	// Now create the actual strategy function using the validated type and delays
	var finalRetryStrategy RetryStrategy
	switch finalStrategyType {
	case FixedDelayStrategy:
		finalRetryStrategy = FixedDelay(b.client.retryBaseDelay)
	case JitterBackoffStrategy:
		finalRetryStrategy = JitterBackoff(b.client.retryBaseDelay, b.client.retryMaxDelay)
	case ExponentialBackoffStrategy:
		finalRetryStrategy = ExponentialBackoff(b.client.retryBaseDelay, b.client.retryMaxDelay)
	default: // Handles invalid types explicitly defaulting to Exponential
		// This case is reached if finalStrategyType was initially invalid ("" or "invalid")
		finalRetryStrategy = ExponentialBackoff(b.client.retryBaseDelay, b.client.retryMaxDelay)
	}

	// Create the underlying standard transport
	transport := &http.Transport{
		MaxIdleConns:          b.client.maxIdleConns,
		IdleConnTimeout:       b.client.idleConnTimeout,
		TLSHandshakeTimeout:   b.client.tlsHandshakeTimeout,
		ExpectContinueTimeout: b.client.expectContinueTimeout,
		DisableKeepAlives:     b.client.disableKeepAlives,
		MaxIdleConnsPerHost:   b.client.maxIdleConnsPerHost,
	}

	// Create retry transport - this is the only layer needed for transparent operation
	// It automatically preserves all existing headers without any explicit auth configuration
	finalTransport := &retryTransport{
		Transport:     transport,
		MaxRetries:    b.client.maxRetries,
		RetryStrategy: finalRetryStrategy,
	}

	// Create the HTTP client with the specified settings
	return &http.Client{
		Timeout:   b.client.timeout,
		Transport: finalTransport,
	}
}
