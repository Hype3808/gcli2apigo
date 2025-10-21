package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"gcli2apigo/internal/banlist"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// CredentialEntry represents a single OAuth credential with associated metadata
type CredentialEntry struct {
	Token     *oauth2.Token
	ProjectID string
	FilePath  string // For logging/debugging purposes
}

// CredentialPool manages multiple OAuth credentials with thread-safe access
type CredentialPool struct {
	credentials []*CredentialEntry
	mu          sync.RWMutex
}

// NewCredentialPool creates a new empty credential pool
func NewCredentialPool() *CredentialPool {
	return &CredentialPool{
		credentials: make([]*CredentialEntry, 0),
	}
}

// AddCredential adds a credential to the pool with write lock
func (cp *CredentialPool) AddCredential(entry *CredentialEntry) error {
	if entry == nil {
		return errors.New("credential entry cannot be nil")
	}
	if entry.Token == nil {
		return errors.New("credential token cannot be nil")
	}
	if entry.ProjectID == "" {
		return errors.New("credential project ID cannot be empty")
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.credentials = append(cp.credentials, entry)
	return nil
}

// GetRandomCredential returns a randomly selected credential from the pool
// Excludes banned credentials from selection
func (cp *CredentialPool) GetRandomCredential() (*CredentialEntry, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	if len(cp.credentials) == 0 {
		fmt.Printf("[ERROR] No credentials available in pool - credential pool is empty\n")
		return nil, errors.New("no credentials available in pool")
	}

	// Filter out banned credentials
	bl := banlist.GetBanList()
	availableCredentials := make([]*CredentialEntry, 0, len(cp.credentials))

	for _, cred := range cp.credentials {
		if !bl.IsBanned(cred.ProjectID) {
			availableCredentials = append(availableCredentials, cred)
		}
	}

	if len(availableCredentials) == 0 {
		fmt.Printf("[ERROR] No unbanned credentials available in pool - total credentials: %d, all are banned\n", len(cp.credentials))
		return nil, errors.New("no unbanned credentials available in pool")
	}

	// Use time-based seed for randomization
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := r.Intn(len(availableCredentials))
	return availableCredentials[idx], nil
}

// Size returns the number of credentials in the pool
func (cp *CredentialPool) Size() int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return len(cp.credentials)
}

// ValidateCredential validates credential JSON data and returns a CredentialEntry
func ValidateCredential(data map[string]interface{}, filePath string) (*CredentialEntry, error) {
	// Check for required fields
	requiredFields := []string{"client_id", "client_secret", "refresh_token", "project_id"}
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			return nil, fmt.Errorf("missing required field: %s", field)
		}
	}

	// Extract values
	clientID, ok := data["client_id"].(string)
	if !ok || clientID == "" {
		return nil, errors.New("client_id must be a non-empty string")
	}

	clientSecret, ok := data["client_secret"].(string)
	if !ok || clientSecret == "" {
		return nil, errors.New("client_secret must be a non-empty string")
	}

	refreshToken, ok := data["refresh_token"].(string)
	if !ok || refreshToken == "" {
		return nil, errors.New("refresh_token must be a non-empty string")
	}

	projectID, ok := data["project_id"].(string)
	if !ok || projectID == "" {
		return nil, errors.New("project_id must be a non-empty string")
	}

	// Extract optional fields
	accessToken, _ := data["token"].(string)
	tokenURI, _ := data["token_uri"].(string)

	// Parse expiry if present
	var expiry time.Time
	if expiryStr, ok := data["expiry"].(string); ok && expiryStr != "" {
		parsedExpiry, err := time.Parse(time.RFC3339, expiryStr)
		if err == nil {
			expiry = parsedExpiry
			fmt.Printf("[DEBUG] Loaded credential with expiry: %s (expired: %v)\n", expiry.Format(time.RFC3339), expiry.Before(time.Now()))
		} else {
			fmt.Printf("Warning: Failed to parse expiry '%s': %v\n", expiryStr, err)
		}
	} else {
		fmt.Printf("Warning: No expiry field found in credential file %s\n", filePath)
	}

	// Create OAuth2 token
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		Expiry:       expiry,
	}

	// Store additional OAuth config data in token extra
	token = token.WithExtra(map[string]interface{}{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"token_uri":     tokenURI,
	})

	entry := &CredentialEntry{
		Token:     token,
		ProjectID: projectID,
		FilePath:  filePath,
	}

	return entry, nil
}

// LoadCredentialsFromFolder loads all valid JSON credential files from a folder
func LoadCredentialsFromFolder(folderPath string, pool *CredentialPool) error {
	// Check if folder exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		fmt.Printf("[ERROR] Credentials folder does not exist: %s\n", folderPath)
		return fmt.Errorf("credentials folder does not exist: %s", folderPath)
	}

	// Read all files in the folder
	files, err := os.ReadDir(folderPath)
	if err != nil {
		fmt.Printf("[ERROR] Failed to read credentials folder %s: %v\n", folderPath, err)
		return fmt.Errorf("failed to read credentials folder: %w", err)
	}

	fmt.Printf("[INFO] Scanning credentials folder: %s (found %d files/directories)\n", folderPath, len(files))

	loadedCount := 0
	skippedCount := 0
	for _, file := range files {
		// Skip directories and non-JSON files
		if file.IsDir() {
			fmt.Printf("[DEBUG] Skipping directory: %s\n", file.Name())
			skippedCount++
			continue
		}
		if filepath.Ext(file.Name()) != ".json" {
			fmt.Printf("[DEBUG] Skipping non-JSON file: %s\n", file.Name())
			skippedCount++
			continue
		}

		filePath := filepath.Join(folderPath, file.Name())
		fmt.Printf("[DEBUG] Processing credential file: %s\n", filePath)

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("[WARN] Failed to read credential file %s: %v\n", filePath, err)
			continue
		}

		// Parse JSON
		var data map[string]interface{}
		if err := json.Unmarshal(content, &data); err != nil {
			fmt.Printf("[WARN] Invalid JSON in credential file %s: %v\n", filePath, err)
			continue
		}

		// Validate and create credential entry
		entry, err := ValidateCredential(data, filePath)
		if err != nil {
			fmt.Printf("[WARN] Invalid credential file %s: %v\n", filePath, err)
			continue
		}

		// Add to pool
		if err := pool.AddCredential(entry); err != nil {
			fmt.Printf("[WARN] Failed to add credential from %s: %v\n", filePath, err)
			continue
		}

		fmt.Printf("[INFO] Successfully loaded credential from %s (project: %s)\n", filePath, entry.ProjectID)
		loadedCount++
	}

	fmt.Printf("[INFO] Loaded %d credential(s) from folder: %s (%d files skipped)\n", loadedCount, folderPath, skippedCount)
	return nil
}

// LoadLegacyCredential loads credentials from legacy sources for backward compatibility
// It checks for oauth_creds.json file and GEMINI_CREDENTIALS environment variable
// Returns the number of credentials loaded from legacy sources
func LoadLegacyCredential(pool *CredentialPool, scriptDir string, folderIsEmpty bool) int {
	loadedCount := 0

	// Load from GEMINI_CREDENTIALS environment variable if set
	if geminiCreds := os.Getenv("GEMINI_CREDENTIALS"); geminiCreds != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(geminiCreds), &data); err != nil {
			fmt.Printf("Warning: Invalid JSON in GEMINI_CREDENTIALS environment variable: %v\n", err)
		} else {
			entry, err := ValidateCredential(data, "GEMINI_CREDENTIALS")
			if err != nil {
				fmt.Printf("Warning: Invalid credential in GEMINI_CREDENTIALS: %v\n", err)
			} else {
				if err := pool.AddCredential(entry); err != nil {
					fmt.Printf("Warning: Failed to add credential from GEMINI_CREDENTIALS: %v\n", err)
				} else {
					fmt.Printf("Loaded credential from GEMINI_CREDENTIALS environment variable\n")
					loadedCount++
				}
			}
		}
	}

	// Load from oauth_creds.json file if folder is empty
	// According to requirement 5.2, prioritize folder credentials over legacy file
	if folderIsEmpty {
		legacyFilePath := filepath.Join(scriptDir, "oauth_creds.json")
		if _, err := os.Stat(legacyFilePath); err == nil {
			// File exists, try to load it
			content, err := os.ReadFile(legacyFilePath)
			if err != nil {
				fmt.Printf("Warning: Failed to read legacy credential file %s: %v\n", legacyFilePath, err)
			} else {
				var data map[string]interface{}
				if err := json.Unmarshal(content, &data); err != nil {
					fmt.Printf("Warning: Invalid JSON in legacy credential file %s: %v\n", legacyFilePath, err)
				} else {
					entry, err := ValidateCredential(data, legacyFilePath)
					if err != nil {
						fmt.Printf("Warning: Invalid legacy credential file %s: %v\n", legacyFilePath, err)
					} else {
						if err := pool.AddCredential(entry); err != nil {
							fmt.Printf("Warning: Failed to add legacy credential from %s: %v\n", legacyFilePath, err)
						} else {
							fmt.Printf("Loaded legacy credential from %s\n", legacyFilePath)
							loadedCount++
						}
					}
				}
			}
		}
	}

	return loadedCount
}
