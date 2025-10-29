package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/httputil"
)

// HandleGoogleAPIsProxy handles Google APIs proxy endpoint
func HandleGoogleAPIsProxy(w http.ResponseWriter, r *http.Request) {
	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","code":401}}`, http.StatusUnauthorized)
		return
	}

	// Only handle Google APIs paths
	if !strings.HasPrefix(r.URL.Path, "/googleapis/") {
		http.NotFound(w, r)
		return
	}

	// Extract the actual API path from /googleapis/{path}
	apiPath := strings.TrimPrefix(r.URL.Path, "/googleapis/")
	if apiPath == "" {
		http.Error(w, `{"error":{"message":"Missing API path after /googleapis/","code":400}}`, http.StatusBadRequest)
		return
	}

	log.Printf("[DEBUG] Google APIs proxy request: method=%s, path=%s", r.Method, apiPath)

	// Get the configured endpoint
	configuredEndpoint := config.GetGoogleAPIsEndpoint()
	log.Printf("[DEBUG] Configured Google APIs endpoint: %s", configuredEndpoint)

	// Get the request body
	var body io.Reader = r.Body
	if r.Body != nil {
		// Read body to allow for potential modification
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":{"message":"Failed to read request body","code":400}}`, http.StatusBadRequest)
			return
		}
		body = bytes.NewReader(bodyBytes)
	}

	// Create the target URL - use dynamic endpoint getter
	targetURL, err := url.Parse(config.GetGoogleAPIsEndpoint())
	if err != nil {
		log.Printf("Failed to parse Google APIs endpoint: %v", err)
		http.Error(w, `{"error":{"message":"Invalid Google APIs endpoint configuration","code":500}}`, http.StatusInternalServerError)
		return
	}

	// Append the API path to the target URL
	targetURL.Path = strings.TrimSuffix(targetURL.Path, "/") + "/" + strings.TrimPrefix(apiPath, "/")
	targetURL.RawQuery = r.URL.RawQuery

	log.Printf("[DEBUG] Final target URL: %s", targetURL.String())

	// Create new request to target
	req, err := http.NewRequest(r.Method, targetURL.String(), body)
	if err != nil {
		log.Printf("Failed to create proxy request: %v", err)
		http.Error(w, `{"error":{"message":"Failed to create proxy request","code":500}}`, http.StatusInternalServerError)
		return
	}

	// Copy headers from original request, excluding some that shouldn't be forwarded
	for name, values := range r.Header {
		// Skip headers that shouldn't be forwarded
		if strings.EqualFold(name, "Host") ||
			strings.EqualFold(name, "Connection") ||
			strings.EqualFold(name, "Content-Length") ||
			strings.EqualFold(name, "Transfer-Encoding") {
			continue
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Set User-Agent to identify the proxy
	req.Header.Set("User-Agent", "gcli2apigo-googleapis-proxy/1.0")

	// Use shared HTTP client (which has proxy configured)
	log.Printf("[DEBUG] Sending request to: %s", req.URL.String())
	resp, err := httputil.SharedHTTPClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Google APIs proxy error: %v", err)
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Proxy error: %v", err),
				"code":    502,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(errorData)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		// Skip headers that shouldn't be forwarded
		if strings.EqualFold(name, "Connection") ||
			strings.EqualFold(name, "Transfer-Encoding") ||
			strings.EqualFold(name, "Content-Length") {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
		return
	}

	log.Printf("Successfully proxied Google APIs request: status=%d, path=%s", resp.StatusCode, apiPath)
}

// HandleGoogleAPIsInfo handles information endpoint for Google APIs proxy
func HandleGoogleAPIsInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":{"message":"Method not allowed","code":405}}`, http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"name":        "Google APIs Proxy",
		"description": "Proxy for Google APIs endpoints",
		"endpoint":    config.GetGoogleAPIsEndpoint(),
		"usage": map[string]string{
			"proxy":   "/googleapis/{api_path}",
			"example": "/googleapis/storage/v1/b",
		},
		"supported_apis": []string{
			"Storage API",
			"Compute Engine API",
			"Cloud Resource Manager API",
			"Service Usage API",
			"And more...",
		},
		"authentication": "Required",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
