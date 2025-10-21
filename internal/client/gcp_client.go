package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

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
	return &GCPClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}
}

// ListProjects retrieves all ACTIVE projects accessible to the authenticated user
// using the Cloud Resource Manager API
func (gc *GCPClient) ListProjects() ([]Project, error) {
	log.Printf("[DEBUG] Starting project discovery using Cloud Resource Manager API")

	allProjects := make([]Project, 0)
	pageToken := ""

	for {
		// Build the API URL
		url := "https://cloudresourcemanager.googleapis.com/v1/projects"
		if pageToken != "" {
			url += "?pageToken=" + pageToken
		}

		// Create the request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to create request for ListProjects: %v", err)
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Add authorization header
		req.Header.Set("Authorization", "Bearer "+gc.token.AccessToken)
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
	url := fmt.Sprintf("https://serviceusage.googleapis.com/v1/projects/%s/services/%s:enable", projectID, serviceName)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to create request for EnableService: %v", err)
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+gc.token.AccessToken)
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
	url := fmt.Sprintf("https://serviceusage.googleapis.com/v1/projects/%s/services/%s", projectID, serviceName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+gc.token.AccessToken)
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
