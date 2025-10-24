package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gcli2apigo/internal/config"
	"gcli2apigo/internal/httputil"
)

// Settings represents the server configuration settings
type Settings struct {
	Host                    string `json:"host"`
	Port                    string `json:"port"`
	Password                string `json:"password,omitempty"` // omitempty to not expose in GET
	MaxRetries              string `json:"max_retries"`
	Proxy                   string `json:"proxy"`
	GeminiEndpoint          string `json:"gemini_endpoint"`
	ResourceManagerEndpoint string `json:"resource_manager_endpoint"`
	ServiceUsageEndpoint    string `json:"service_usage_endpoint"`
	OAuth2Endpoint          string `json:"oauth2_endpoint"`
	GoogleApisEndpoint      string `json:"google_apis_endpoint"`
}

// HandleGetSettings returns the current server settings (excluding password)
func (dh *DashboardHandlers) HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxyValue := os.Getenv("HTTP_PROXY")
	settings := Settings{
		Host:                    os.Getenv("HOST"),
		Port:                    os.Getenv("PORT"),
		MaxRetries:              os.Getenv("MAX_RETRY_ATTEMPTS"),
		Proxy:                   proxyValue,
		GeminiEndpoint:          os.Getenv("GEMINI_API_ENDPOINT"),
		ResourceManagerEndpoint: os.Getenv("GCP_RESOURCE_MANAGER_ENDPOINT"),
		ServiceUsageEndpoint:    os.Getenv("GCP_SERVICE_USAGE_ENDPOINT"),
		OAuth2Endpoint:          os.Getenv("OAUTH2_ENDPOINT"),
		GoogleApisEndpoint:      os.Getenv("GOOGLE_APIS_ENDPOINT"),
	}

	// Set defaults if empty
	if settings.Host == "" {
		settings.Host = "0.0.0.0"
	}
	if settings.Port == "" {
		settings.Port = "7860"
	}
	if settings.MaxRetries == "" {
		settings.MaxRetries = "5"
	}
	if settings.GeminiEndpoint == "" {
		settings.GeminiEndpoint = "https://cloudcode-pa.googleapis.com"
	}
	if settings.ResourceManagerEndpoint == "" {
		settings.ResourceManagerEndpoint = "https://cloudresourcemanager.googleapis.com"
	}
	if settings.ServiceUsageEndpoint == "" {
		settings.ServiceUsageEndpoint = "https://serviceusage.googleapis.com"
	}
	if settings.OAuth2Endpoint == "" {
		settings.OAuth2Endpoint = "https://oauth2.googleapis.com"
	}
	if settings.GoogleApisEndpoint == "" {
		settings.GoogleApisEndpoint = "https://www.googleapis.com"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"settings": settings,
	})
}

// HandleSaveSettings saves the server settings to .env file
func (dh *DashboardHandlers) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var settings Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		log.Printf("[ERROR] Failed to decode settings: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Validate settings
	if settings.Port != "" {
		// Basic port validation
		if len(settings.Port) == 0 || len(settings.Port) > 5 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid port number",
			})
			return
		}
	}

	// Read existing .env file or create new one
	envPath := ".env"
	envVars := make(map[string]string)

	// Try to read existing .env file
	if data, err := os.ReadFile(envPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present
				value = strings.Trim(value, `"'`)
				envVars[key] = value
			}
		}
	}

	// Update settings - only update if value is provided (not empty)
	// Existing values in envVars are preserved if not updated
	// This allows partial updates without losing other settings
	if settings.Host != "" {
		envVars["HOST"] = settings.Host
	}
	if settings.Port != "" {
		envVars["PORT"] = settings.Port
	}
	if settings.Password != "" {
		envVars["GEMINI_AUTH_PASSWORD"] = settings.Password
	}
	if settings.MaxRetries != "" {
		envVars["MAX_RETRY_ATTEMPTS"] = settings.MaxRetries
	}

	// Handle proxy settings - support both setting and clearing
	// When proxy is empty string, we explicitly remove it from envVars
	proxyChanged := false
	oldProxy := envVars["HTTP_PROXY"]
	if settings.Proxy != "" {
		// Set new proxy
		envVars["HTTP_PROXY"] = settings.Proxy
		envVars["HTTPS_PROXY"] = settings.Proxy
		proxyChanged = (oldProxy != settings.Proxy)
		log.Printf("[INFO] Proxy updated to: %s", settings.Proxy)
	} else {
		// Clear proxy - delete from envVars map so it won't be written to .env
		if oldProxy != "" {
			delete(envVars, "HTTP_PROXY")
			delete(envVars, "HTTPS_PROXY")
			proxyChanged = true
			log.Printf("[INFO] Proxy cleared (was: %s)", oldProxy)
		}
	}

	// Update API endpoint settings
	if settings.GeminiEndpoint != "" {
		envVars["GEMINI_API_ENDPOINT"] = settings.GeminiEndpoint
	}
	if settings.ResourceManagerEndpoint != "" {
		envVars["GCP_RESOURCE_MANAGER_ENDPOINT"] = settings.ResourceManagerEndpoint
	}
	if settings.ServiceUsageEndpoint != "" {
		envVars["GCP_SERVICE_USAGE_ENDPOINT"] = settings.ServiceUsageEndpoint
	}
	if settings.OAuth2Endpoint != "" {
		envVars["OAUTH2_ENDPOINT"] = settings.OAuth2Endpoint
	}
	if settings.GoogleApisEndpoint != "" {
		envVars["GOOGLE_APIS_ENDPOINT"] = settings.GoogleApisEndpoint
	}

	log.Printf("[DEBUG] Saving settings to .env: %v", envVars)

	// Write back to .env file
	var envContent strings.Builder
	envContent.WriteString("# Server Configuration\n")
	envContent.WriteString("# Generated by gcli2apigo dashboard\n\n")

	// Write in a consistent order
	keys := []string{
		"HOST", "PORT", "GEMINI_AUTH_PASSWORD", "MAX_RETRY_ATTEMPTS",
		"HTTP_PROXY", "HTTPS_PROXY",
		"GEMINI_API_ENDPOINT", "GCP_RESOURCE_MANAGER_ENDPOINT",
		"GCP_SERVICE_USAGE_ENDPOINT", "OAUTH2_ENDPOINT", "GOOGLE_APIS_ENDPOINT",
	}
	for _, key := range keys {
		if value, exists := envVars[key]; exists {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	// Write any other variables that might exist
	for key, value := range envVars {
		found := false
		for _, k := range keys {
			if k == key {
				found = true
				break
			}
		}
		if !found {
			envContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		log.Printf("[ERROR] Failed to create directory for .env: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save settings",
		})
		return
	}

	// Write to file
	if err := os.WriteFile(envPath, []byte(envContent.String()), 0600); err != nil {
		log.Printf("[ERROR] Failed to write .env file: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save settings",
		})
		return
	}

	log.Printf("[INFO] Settings saved successfully to %s", envPath)

	// Update in-memory config for settings that don't require restart
	if settings.Password != "" {
		config.GeminiAuthPassword = settings.Password
		log.Printf("[INFO] Password updated in memory")
	}
	if settings.MaxRetries != "" {
		os.Setenv("MAX_RETRY_ATTEMPTS", settings.MaxRetries)
		log.Printf("[INFO] Max retry attempts updated in memory: %s", settings.MaxRetries)
	}

	// Update proxy environment variables and recreate HTTP client if proxy changed
	if proxyChanged {
		if settings.Proxy != "" {
			// Set proxy in environment
			os.Setenv("HTTP_PROXY", settings.Proxy)
			os.Setenv("HTTPS_PROXY", settings.Proxy)
			os.Setenv("http_proxy", settings.Proxy)
			os.Setenv("https_proxy", settings.Proxy)
			log.Printf("[INFO] Proxy environment variables updated to: %s", settings.Proxy)
		} else {
			// Clear proxy from environment
			os.Unsetenv("HTTP_PROXY")
			os.Unsetenv("HTTPS_PROXY")
			os.Unsetenv("http_proxy")
			os.Unsetenv("https_proxy")
			log.Printf("[INFO] Proxy environment variables cleared")
		}

		// Recreate HTTP client to apply new proxy settings
		httputil.RecreateHTTPClient()
		log.Printf("[INFO] HTTP client recreated with new proxy settings")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Settings saved successfully",
	})
}
