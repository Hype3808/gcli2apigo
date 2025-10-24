package httputil

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
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

	// Configure proxy if HTTP_PROXY or HTTPS_PROXY environment variable is set
	if proxyURL := getProxyURL(); proxyURL != nil {
		if strings.HasPrefix(proxyURL.Scheme, "socks5") {
			// SOCKS5 proxy requires special handling
			configureSocks5Proxy(transport, proxyURL)
		} else {
			// HTTP/HTTPS proxy
			transport.Proxy = http.ProxyURL(proxyURL)
		}
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

// getProxyURL returns the proxy URL from environment variables
// Checks HTTP_PROXY and HTTPS_PROXY (case-insensitive)
// Supports: http://, https://, socks5://, socks5h://
func getProxyURL() *url.URL {
	// Check for proxy environment variables (case-insensitive)
	proxyStr := os.Getenv("HTTPS_PROXY")
	if proxyStr == "" {
		proxyStr = os.Getenv("https_proxy")
	}
	if proxyStr == "" {
		proxyStr = os.Getenv("HTTP_PROXY")
	}
	if proxyStr == "" {
		proxyStr = os.Getenv("http_proxy")
	}

	if proxyStr == "" {
		return nil
	}

	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		log.Printf("[WARN] Invalid proxy URL '%s': %v", proxyStr, err)
		return nil
	}

	// Validate proxy scheme
	scheme := strings.ToLower(proxyURL.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "socks5" && scheme != "socks5h" {
		log.Printf("[WARN] Unsupported proxy scheme '%s'. Supported: http, https, socks5, socks5h", scheme)
		return nil
	}

	log.Printf("[INFO] Using %s proxy: %s://%s", strings.ToUpper(scheme), scheme, proxyURL.Host)
	return proxyURL
}

// configureSocks5Proxy configures SOCKS5 proxy for the transport
func configureSocks5Proxy(transport *http.Transport, proxyURL *url.URL) {
	// Create SOCKS5 dialer
	var auth *proxy.Auth
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     proxyURL.User.Username(),
			Password: password,
		}
	}

	// Create SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		log.Printf("[ERROR] Failed to create SOCKS5 proxy dialer: %v", err)
		return
	}

	// Set custom DialContext that uses SOCKS5
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}

	log.Printf("[INFO] SOCKS5 proxy configured successfully")
}

// RecreateHTTPClient recreates the shared HTTP client with current environment settings
// This should be called after proxy settings are changed
func RecreateHTTPClient() {
	SharedHTTPClient = createHTTPClient()
}
