package httputil

import (
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

var (
	// SharedHTTPClient is the optimized HTTP/2 client for all API requests.
	// It is configured with HTTP/2 and connection pooling to support high throughput.
	SharedHTTPClient = createHTTPClient()
)

// createHTTPClient creates and configures the HTTP/2 client
func createHTTPClient() *http.Client {
	// Create HTTP/2 transport with optimized settings
	transport := &http.Transport{
		// Maximum number of idle connections across all hosts
		MaxIdleConns: 200,

		// Maximum number of idle connections per host
		MaxIdleConnsPerHost: 100,

		// How long an idle connection remains in the pool before being closed
		IdleConnTimeout: 90 * time.Second,

		// Keep-alive connections enabled for connection pooling
		DisableKeepAlives: false,

		// Compression enabled for bandwidth efficiency
		DisableCompression: false,

		// Force HTTP/2 for all requests
		ForceAttemptHTTP2: true,

		// Optimized buffer sizes for streaming
		// 64KB write buffer for efficient chunk accumulation
		WriteBufferSize: 64 * 1024,
		// 256KB read buffer for efficient response parsing
		ReadBufferSize: 256 * 1024,
	}

	// Configure HTTP/2 explicitly
	if err := http2.ConfigureTransport(transport); err != nil {
		// If HTTP/2 configuration fails, panic as HTTP/2 is required
		panic("Failed to configure HTTP/2: " + err.Error())
	}

	return &http.Client{
		// Increased timeout to handle long-running requests with large responses
		// Streaming responses can take several minutes for maximum token outputs
		Timeout:   10 * time.Minute,
		Transport: transport,
	}
}
