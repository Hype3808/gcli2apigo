package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gcli2apigo/internal/banlist"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/usage"
)

// CredentialInfo represents metadata about a stored OAuth credential
type CredentialInfo struct {
	ProjectID     string    `json:"project_id"`
	ClientID      string    `json:"client_id"`
	CreatedAt     time.Time `json:"created_at"`
	ModifiedAt    time.Time `json:"modified_at"`
	FilePath      string    `json:"-"` // Internal use only, not exposed in JSON
	ProModelCount int       `json:"pro_model_count"`
	ProModelLimit int       `json:"pro_model_limit"`
	OverallCount  int       `json:"overall_count"`
	OverallLimit  int       `json:"overall_limit"`
	NextResetTime time.Time `json:"next_reset_time"`
	IsBanned      bool      `json:"is_banned"`
	LastErrorCode int       `json:"last_error_code"` // HTTP error code from last API request, 0 if successful
	Expiry        time.Time `json:"expiry"`          // OAuth token expiry time
}

// ListCredentials scans the oauth_creds directory and returns information about all credential files
func ListCredentials() ([]CredentialInfo, error) {
	credentialsDir := config.OAuthCredsFolder

	log.Printf("[DEBUG] Listing credentials from directory: %s", credentialsDir)

	// Check if directory exists
	if _, err := os.Stat(credentialsDir); os.IsNotExist(err) {
		// Directory doesn't exist, return empty list
		log.Printf("[INFO] Credentials directory does not exist: %s", credentialsDir)
		return []CredentialInfo{}, nil
	}

	// Read all files in the directory
	files, err := os.ReadDir(credentialsDir)
	if err != nil {
		log.Printf("[ERROR] Failed to read credentials directory %s: %v", credentialsDir, err)
		return nil, fmt.Errorf("failed to read credentials directory: %w", err)
	}

	credentials := make([]CredentialInfo, 0)
	skippedFiles := 0

	for _, file := range files {
		// Skip directories and non-JSON files
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			skippedFiles++
			continue
		}

		filePath := filepath.Join(credentialsDir, file.Name())

		// Get credential info from file
		credInfo, err := GetCredentialInfo(filePath)
		if err != nil {
			// Log error but continue processing other files
			log.Printf("[WARN] Failed to process credential file %s: %v", filePath, err)
			skippedFiles++
			continue
		}

		credentials = append(credentials, *credInfo)
	}

	log.Printf("[INFO] Successfully listed %d credentials (%d files skipped)", len(credentials), skippedFiles)
	return credentials, nil
}

// GetCredentialInfo extracts metadata from a credential file
func GetCredentialInfo(filePath string) (*CredentialInfo, error) {
	log.Printf("[DEBUG] Reading credential info from: %s", filePath)

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[ERROR] Failed to read credential file %s: %v", filePath, err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON to extract project_id and client_id
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		log.Printf("[ERROR] Invalid JSON format in credential file %s: %v", filePath, err)
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Extract project_id
	projectID, ok := data["project_id"].(string)
	if !ok || projectID == "" {
		log.Printf("[ERROR] Missing or invalid project_id in credential file %s", filePath)
		return nil, fmt.Errorf("missing or invalid project_id field")
	}

	// Extract client_id
	clientID, ok := data["client_id"].(string)
	if !ok || clientID == "" {
		log.Printf("[ERROR] Missing or invalid client_id in credential file %s", filePath)
		return nil, fmt.Errorf("missing or invalid client_id field")
	}

	// Get file metadata for timestamps
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("[ERROR] Failed to get file info for %s: %v", filePath, err)
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get usage statistics
	tracker := usage.GetTracker()
	usageStats := tracker.GetUsage(projectID)

	// Check ban status
	banList := banlist.GetBanList()
	isBanned := banList.IsBanned(projectID)

	// Get last error code
	lastErrorCode := tracker.GetLastErrorCode(projectID)

	// Extract expiry time
	var expiry time.Time
	if expiryStr, ok := data["expiry"].(string); ok && expiryStr != "" {
		// Try to parse the expiry time
		if parsedExpiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
			expiry = parsedExpiry
		} else {
			log.Printf("[WARN] Failed to parse expiry time for %s: %v", projectID, err)
		}
	}

	credInfo := &CredentialInfo{
		ProjectID:     projectID,
		ClientID:      clientID,
		CreatedAt:     fileInfo.ModTime(), // Use ModTime as creation time (best available on all platforms)
		ModifiedAt:    fileInfo.ModTime(),
		FilePath:      filePath,
		ProModelCount: usageStats.ProModelCount,
		ProModelLimit: usage.ProModelDailyLimit,
		OverallCount:  usageStats.OverallCount,
		OverallLimit:  usage.OverallDailyLimit,
		NextResetTime: tracker.GetNextResetTime(),
		IsBanned:      isBanned,
		LastErrorCode: lastErrorCode,
		Expiry:        expiry,
	}

	log.Printf("[DEBUG] Successfully extracted credential info for project: %s (usage: %d/%d pro, %d/%d overall)",
		projectID, usageStats.ProModelCount, usage.ProModelDailyLimit, usageStats.OverallCount, usage.OverallDailyLimit)
	return credInfo, nil
}

// ValidateProjectID validates a project ID to prevent path traversal and injection attacks
// This function implements multiple security checks:
// - Prevents empty or invalid length project IDs
// - Blocks path traversal attempts (.. sequences)
// - Blocks directory separators (/ and \)
// - Blocks null bytes (security risk)
// - Enforces GCP project ID naming conventions (alphanumeric + hyphens, starts with letter)
// Returns an error if the project ID is invalid
func ValidateProjectID(projectID string) error {
	// Check for empty project ID
	if projectID == "" {
		return fmt.Errorf("project_id cannot be empty")
	}

	// Check length (GCP project IDs are 6-30 characters)
	if len(projectID) < 6 || len(projectID) > 30 {
		return fmt.Errorf("project_id must be between 6 and 30 characters")
	}

	// Check for path traversal attempts
	if strings.Contains(projectID, "..") {
		return fmt.Errorf("project_id contains path traversal sequence")
	}

	// Check for directory separators (both Unix and Windows)
	if strings.ContainsAny(projectID, "/\\") {
		return fmt.Errorf("project_id contains directory separator")
	}

	// Check for null bytes (security risk)
	if strings.Contains(projectID, "\x00") {
		return fmt.Errorf("project_id contains null byte")
	}

	// GCP project IDs must start with a lowercase letter and contain only lowercase letters, digits, and hyphens
	// We'll be more permissive here to support various naming conventions, but still prevent obvious attacks
	for i, c := range projectID {
		if i == 0 {
			// First character must be a letter
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return fmt.Errorf("project_id must start with a letter")
			}
		} else {
			// Subsequent characters must be alphanumeric or hyphen
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return fmt.Errorf("project_id contains invalid character: %c", c)
			}
		}
	}

	return nil
}

// DeleteCredential deletes a credential file by project ID
func DeleteCredential(projectID string) error {
	log.Printf("[INFO] Attempting to delete credential for project: %s", projectID)

	// Validate project_id to prevent path traversal and injection attacks
	if err := ValidateProjectID(projectID); err != nil {
		log.Printf("[ERROR] Delete credential failed: invalid project_id: %v", err)
		return fmt.Errorf("invalid project_id: %w", err)
	}

	// Construct file path
	credentialsDir := config.OAuthCredsFolder
	filePath := filepath.Join(credentialsDir, projectID+".json")

	// Additional security check: Ensure the resolved path is still within the credentials directory
	// This prevents symlink attacks and other path manipulation
	absCredsDir, err := filepath.Abs(credentialsDir)
	if err != nil {
		log.Printf("[ERROR] Failed to resolve credentials directory path: %v", err)
		return fmt.Errorf("failed to resolve credentials directory: %w", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("[ERROR] Failed to resolve file path: %v", err)
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check if the file path is within the credentials directory
	if !strings.HasPrefix(absFilePath, absCredsDir+string(filepath.Separator)) {
		log.Printf("[ERROR] Security violation: attempted to delete file outside credentials directory: %s", absFilePath)
		return fmt.Errorf("security violation: file path outside credentials directory")
	}

	log.Printf("[DEBUG] Deleting credential file: %s", filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("[ERROR] Credential file not found: %s", filePath)
		return fmt.Errorf("credential file not found for project: %s", projectID)
	}

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		log.Printf("[ERROR] Failed to delete credential file %s: %v", filePath, err)
		return fmt.Errorf("failed to delete credential file: %w", err)
	}

	log.Printf("[INFO] Successfully deleted credential file for project: %s", projectID)
	return nil
}

// DashboardStats represents statistics for the dashboard
type DashboardStats struct {
	TotalProRequests     int       `json:"total_pro_requests"`
	TotalOverallRequests int       `json:"total_overall_requests"`
	RPM                  float64   `json:"rpm"`
	ActiveCredentials    int       `json:"active_credentials"`
	NextResetTime        time.Time `json:"next_reset_time"`
}

// GetDashboardStats calculates and returns dashboard statistics
func GetDashboardStats() DashboardStats {
	tracker := usage.GetTracker()
	allUsage := tracker.GetAllUsage()
	banList := banlist.GetBanList()

	totalProRequests := 0
	totalOverallRequests := 0
	activeCredentials := 0

	// Calculate totals and count active credentials
	for projectID, usageStats := range allUsage {
		totalProRequests += usageStats.ProModelCount
		totalOverallRequests += usageStats.OverallCount

		// Count as active if not banned
		if !banList.IsBanned(projectID) {
			activeCredentials++
		}
	}

	// Calculate RPM (requests per minute since last reset)
	nextResetTime := tracker.GetNextResetTime()
	lastResetTime := nextResetTime.Add(-24 * time.Hour)
	minutesSinceReset := time.Since(lastResetTime).Minutes()

	rpm := 0.0
	if minutesSinceReset > 0 {
		rpm = float64(totalOverallRequests) / minutesSinceReset
	}

	return DashboardStats{
		TotalProRequests:     totalProRequests,
		TotalOverallRequests: totalOverallRequests,
		RPM:                  rpm,
		ActiveCredentials:    activeCredentials,
		NextResetTime:        nextResetTime,
	}
}
