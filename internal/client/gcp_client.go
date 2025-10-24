package client

import (
	"context"
	"encoding/json"
	"fmt"
	"gcli2apigo/internal/config"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
	"golang.org/x/oauth2"
)

// GCPClient handles interactions with Google Cloud Platform APIs
type GCPClient struct {
	httpClient *http.Client
	token      *oauth2.Token
}

// Project represents a Google Cloud Platform project
type Project struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"name"`
	State       string `json:"lifecycleState"`
}

// projectsResponse represents the response from Cloud Resource Manager API
type projectsResponse struct {
	Projects      []Project `json:"projects"`
	NextPageToken string    `json:"nextPageToken"`
}

// NewGCPClient creates a new GCP client with the provided OAuth token
func NewGCPClient(token *oauth2.Token) *GCPClient {
	// Create transport with proxy support
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     5 * time.Minute,
	}

	// Configure proxy if environment variable is set
	if proxyURL := getProxyURL(); proxyURL != nil {
		if strings.HasPrefix(proxyURL.Scheme, "socks5") {
			// SOCKS5 proxy requires special handling
			configureSocks5ProxyForGCP(transport, proxyURL)
		} else {
			// HTTP/HTTPS proxy
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	return &GCPClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		token: token,
	}
}

// getProxyURL returns the proxy URL from environment variables
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

	log.Printf("[INFO] GCP client using %s proxy: %s://%s", strings.ToUpper(scheme), scheme, proxyURL.Host)
	return proxyURL
}

// configureSocks5ProxyForGCP configures SOCKS5 proxy for GCP client transport
func configureSocks5ProxyForGCP(transport *http.Transport, proxyURL *url.URL) {
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
		log.Printf("[ERROR] Failed to create SOCKS5 proxy dialer for GCP client: %v", err)
		return
	}

	// Set custom DialContext that uses SOCKS5
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}

	log.Printf("[INFO] GCP client SOCKS5 proxy configured successfully")
}

// ListProjects retrieves all ACTIVE projects accessible to the authenticated user
// using the Cloud Resource Manager API
func (gc *GCPClient) ListProjects() ([]Project, error) {
	log.Printf("[DEBUG] Starting project discovery using Cloud Resource Manager API")

	allProjects := make([]Project, 0)
	pageToken := ""

	for {
		// Build the API URL using strings.Builder to avoid allocations
		var urlBuilder strings.Builder
		urlBuilder.WriteString(config.CloudResourceManagerEndpoint)
		urlBuilder.WriteString("/v1/projects")
		if pageToken != "" {
			urlBuilder.WriteString("?pageToken=")
			urlBuilder.WriteString(pageToken)
		}
		url := urlBuilder.String()

		// Create the request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to create request for ListProjects: %v", err)
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Add authorization header using strings.Builder to avoid allocation
		var authHeader strings.Builder
		authHeader.WriteString("Bearer ")
		authHeader.WriteString(gc.token.AccessToken)
		req.Header.Set("Authorization", authHeader.String())
		req.Header.Set("Content-Type", "application/json")

		// Send the request
		resp, err := gc.httpClient.Do(req)
		if err != nil {
			log.Printf("[ERROR] Failed to execute ListProjects request: %v", err)
			return nil, fmt.Errorf("failed to execute request: %v", err)
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[ERROR] Failed to read ListProjects response: %v", err)
			return nil, fmt.Errorf("failed to read response: %v", err)
		}

		// Check for API errors
		if resp.StatusCode != http.StatusOK {
			log.Printf("[ERROR] Cloud Resource Manager API returned status %d: %s", resp.StatusCode, string(body))
			return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
		}

		// Parse the response
		var projectsResp projectsResponse
		if err := json.Unmarshal(body, &projectsResp); err != nil {
			log.Printf("[ERROR] Failed to parse ListProjects response: %v", err)
			return nil, fmt.Errorf("failed to parse response: %v", err)
		}

		// Filter for ACTIVE projects only
		for _, project := range projectsResp.Projects {
			if project.State == "ACTIVE" {
				allProjects = append(allProjects, project)
				log.Printf("[DEBUG] Found ACTIVE project: %s (name: %s)", project.ProjectID, project.ProjectName)
			} else {
				log.Printf("[DEBUG] Skipping project %s with state: %s", project.ProjectID, project.State)
			}
		}

		// Check if there are more pages
		if projectsResp.NextPageToken == "" {
			break
		}
		pageToken = projectsResp.NextPageToken
	}

	log.Printf("[DEBUG] Project discovery complete. Found %d ACTIVE projects", len(allProjects))
	return allProjects, nil
}

// serviceResponse represents the response from Service Usage API when getting a service
type serviceResponse struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// EnableService enables a specific API service for a given project
// It checks if the service is already enabled and skips if so
// Returns nil on success or if already enabled, error otherwise
func (gc *GCPClient) EnableService(projectID string, serviceName string) error {
	log.Printf("[DEBUG] Checking if service %s is enabled for project %s", serviceName, projectID)

	// First, check if the service is already enabled
	isEnabled, err := gc.isServiceEnabled(projectID, serviceName)
	if err != nil {
		log.Printf("[WARN] Failed to check service status for %s in project %s: %v", serviceName, projectID, err)
		// Continue with enablement attempt even if check fails
	} else if isEnabled {
		log.Printf("[DEBUG] Service %s is already enabled for project %s, skipping", serviceName, projectID)
		return nil
	}

	// Enable the service
	url := fmt.Sprintf("%s/v1/projects/%s/services/%s:enable", config.ServiceUsageEndpoint, projectID, serviceName)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create request for EnableService: %v", err)
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add authorization header using strings.Builder to avoid allocation
	var authHeader strings.Builder
	authHeader.WriteString("Bearer ")
	authHeader.WriteString(gc.token.AccessToken)
	req.Header.Set("Authorization", authHeader.String())
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := gc.httpClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Failed to execute EnableService request for %s in project %s: %v", serviceName, projectID, err)
		return fmt.Errorf("failed to execute request: %v", err)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Printf("[ERROR] Failed to read EnableService response: %v", err)
		return fmt.Errorf("failed to read response: %v", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		log.Printf("[ERROR] Service Usage API returned status %d for %s in project %s: %s", resp.StatusCode, serviceName, projectID, string(body))
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	log.Printf("[DEBUG] Successfully enabled service %s for project %s", serviceName, projectID)
	return nil
}

// isServiceEnabled checks if a service is already enabled for a project
func (gc *GCPClient) isServiceEnabled(projectID string, serviceName string) (bool, error) {
	url := fmt.Sprintf("%s/v1/projects/%s/services/%s", config.ServiceUsageEndpoint, projectID, serviceName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	// Add authorization header using strings.Builder to avoid allocation
	var authHeader strings.Builder
	authHeader.WriteString("Bearer ")
	authHeader.WriteString(gc.token.AccessToken)
	req.Header.Set("Authorization", authHeader.String())
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := gc.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %v", err)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return false, fmt.Errorf("failed to read response: %v", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var serviceResp serviceResponse
	if err := json.Unmarshal(body, &serviceResp); err != nil {
		return false, fmt.Errorf("failed to parse response: %v", err)
	}

	// Check if the service is enabled
	return serviceResp.State == "ENABLED", nil
}

// EnableGeminiAPIs enables both required Gemini APIs for a project
// Returns a map of service names to error (nil if successful)
func (gc *GCPClient) EnableGeminiAPIs(projectID string) map[string]error {
	results := make(map[string]error)

	// List of required Gemini APIs
	services := []string{
		"cloudaicompanion.googleapis.com",   // Gemini Cloud Assist API
		"generativelanguage.googleapis.com", // Gemini for Google Cloud API
	}

	log.Printf("[DEBUG] Enabling Gemini APIs for project %s", projectID)

	for _, service := range services {
		err := gc.EnableService(projectID, service)
		results[service] = err
		if err != nil {
			log.Printf("[ERROR] Failed to enable %s for project %s: %v", service, projectID, err)
		} else {
			log.Printf("[DEBUG] Successfully enabled %s for project %s", service, projectID)
		}
	}

	return results
}
