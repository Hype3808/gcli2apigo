package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gcli2apigo/internal/config"
	"gcli2apigo/internal/httputil"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	userProjectID   string
	oauthConfig     *oauth2.Config
	credentialPool  *CredentialPool
	rateLimitedPool *RateLimitedCredentialPool
	onboardingCache *OnboardingCache
)

func init() {
	oauthConfig = &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080",
		Scopes:       config.Scopes,
	}
}

// AuthenticateUser authenticates the user with API key
// Priority: PASSWORD (if set, overrides all) > GEMINI_API_KEY
func AuthenticateUser(r *http.Request) (string, error) {
	// Check for API key in query parameters first
	apiKey := r.URL.Query().Get("key")
	if apiKey != "" {
		if isValidAPIKey(apiKey) {
			return "api_key_user", nil
		}
	}

	// Check for API key in x-goog-api-key header
	googAPIKey := r.Header.Get("x-goog-api-key")
	if googAPIKey != "" {
		if isValidAPIKey(googAPIKey) {
			return "goog_api_key_user", nil
		}
	}

	// Check for API key in Authorization header (Bearer token format)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
		if isValidAPIKey(bearerToken) {
			return "bearer_user", nil
		}
	}

	// Check for HTTP Basic Authentication
	if strings.HasPrefix(authHeader, "Basic ") {
		encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
		decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredentials)
		if err == nil {
			decoded := string(decodedBytes)
			parts := strings.SplitN(decoded, ":", 2)
			if len(parts) == 2 {
				username, password := parts[0], parts[1]
				if isValidAPIKey(password) {
					return username, nil
				}
			}
		}
	}

	return "", errors.New("invalid authentication credentials")
}

// isValidAPIKey checks if the provided key is valid for API authentication
// Priority: PASSWORD (if set, overrides all) > GEMINI_API_KEY
func isValidAPIKey(key string) bool {
	// If PASSWORD is set, it overrides everything for API authentication
	if config.Password != "" {
		return key == config.Password
	}

	// Otherwise check GEMINI_API_KEY
	if config.GeminiAPIKey != "" && key == config.GeminiAPIKey {
		return true
	}

	return false
}

// SaveCredentials saves credentials to file (used for updating project_id in existing files)
func SaveCredentials(token *oauth2.Token, projectID string) error {
	if projectID != "" && fileExists(config.CredentialFile) {
		data, err := os.ReadFile(config.CredentialFile)
		if err == nil {
			var existingData map[string]interface{}
			if err := json.Unmarshal(data, &existingData); err == nil {
				if _, ok := existingData["project_id"]; !ok {
					existingData["project_id"] = projectID
					updatedData, _ := json.MarshalIndent(existingData, "", "  ")
					os.WriteFile(config.CredentialFile, updatedData, 0600)
					log.Printf("Added project_id %s to existing credential file", projectID)
				}
			}
		}
	}
	return nil
}

// SaveRefreshedToken saves a refreshed OAuth token back to its credential file
func SaveRefreshedToken(credEntry *CredentialEntry) error {
	if credEntry == nil || credEntry.FilePath == "" {
		return errors.New("invalid credential entry")
	}

	// Read existing credential file
	data, err := os.ReadFile(credEntry.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read credential file: %v", err)
	}

	var credData map[string]interface{}
	if err := json.Unmarshal(data, &credData); err != nil {
		return fmt.Errorf("failed to parse credential file: %v", err)
	}

	// Update token fields
	credData["access_token"] = credEntry.Token.AccessToken
	credData["token_type"] = credEntry.Token.TokenType
	newExpiry := credEntry.Token.Expiry.Format(time.RFC3339)
	credData["expiry"] = newExpiry

	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Updating credential file with new expiry: %s", newExpiry)
	}

	// Only update refresh token if it's present in the new token
	if credEntry.Token.RefreshToken != "" {
		credData["refresh_token"] = credEntry.Token.RefreshToken
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(credData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated credentials: %v", err)
	}

	if err := os.WriteFile(credEntry.FilePath, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write credential file: %v", err)
	}

	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Saved refreshed token to: %s", credEntry.FilePath)
	}
	return nil
}

// SaveRefreshedTokenAsync saves a refreshed token asynchronously without blocking
func SaveRefreshedTokenAsync(credEntry *CredentialEntry) {
	go func() {
		if err := SaveRefreshedToken(credEntry); err != nil {
			log.Printf("[WARN] Failed to save refreshed token asynchronously: %v", err)
		}
	}()
}

// OnboardUser ensures the user is onboarded
func OnboardUser(token *oauth2.Token, projectID string) error {
	// Check cache first to avoid redundant API calls
	if onboardingCache != nil && onboardingCache.IsOnboarded(projectID) {
		if config.IsDebugEnabled() {
			log.Printf("[DEBUG] Project %s already onboarded (cached)", projectID)
		}
		return nil
	}

	// Refresh token if expired
	if token.Expiry.Before(time.Now()) && token.RefreshToken != "" {
		if config.IsDebugEnabled() {
			log.Printf("[DEBUG] Token expired in OnboardUser, refreshing...")
		}

		// Extract client credentials from token extra data or use defaults
		clientID := config.ClientID
		clientSecret := config.ClientSecret
		if extra := token.Extra("client_id"); extra != nil {
			if id, ok := extra.(string); ok && id != "" {
				clientID = id
			}
		}
		if extra := token.Extra("client_secret"); extra != nil {
			if secret, ok := extra.(string); ok && secret != "" {
				clientSecret = secret
			}
		}

		tokenConfig := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: config.GetOAuth2Endpoint() + "/token",
			},
		}

		log.Printf("[DEBUG] OnboardUser token refresh - Token URL: %s", tokenConfig.Endpoint.TokenURL)

		newToken, err := tokenConfig.TokenSource(context.Background(), token).Token()
		if err != nil {
			return fmt.Errorf("failed to refresh credentials during onboarding: %v", err)
		}

		if config.IsDebugEnabled() {
			log.Printf("[DEBUG] Token refreshed successfully in OnboardUser")
		}
		*token = *newToken
		SaveCredentials(token, "")
	}

	// Load code assist
	loadAssistPayload := map[string]interface{}{
		"cloudaicompanionProject": projectID,
		"metadata":                config.GetClientMetadata(projectID),
	}

	loadData, err := makeAPIRequest(token, "/v1internal:loadCodeAssist", loadAssistPayload)
	if err != nil {
		return fmt.Errorf("user onboarding failed: %v", err)
	}

	// Determine tier
	var tier map[string]interface{}
	if currentTier, ok := loadData["currentTier"].(map[string]interface{}); ok {
		tier = currentTier
	} else {
		allowedTiers, _ := loadData["allowedTiers"].([]interface{})
		for _, t := range allowedTiers {
			if tierMap, ok := t.(map[string]interface{}); ok {
				if isDefault, _ := tierMap["isDefault"].(bool); isDefault {
					tier = tierMap
					break
				}
			}
		}

		if tier == nil {
			tier = map[string]interface{}{
				"name":                               "",
				"description":                        "",
				"id":                                 "legacy-tier",
				"userDefinedCloudaicompanionProject": true,
			}
		}
	}

	if userDefined, _ := tier["userDefinedCloudaicompanionProject"].(bool); userDefined && projectID == "" {
		return errors.New("this account requires setting the GOOGLE_CLOUD_PROJECT env var")
	}

	if _, ok := loadData["currentTier"]; ok {
		// Mark project as onboarded in cache
		if onboardingCache != nil {
			onboardingCache.MarkOnboarded(projectID)
			if config.IsDebugEnabled() {
				log.Printf("[DEBUG] Project %s marked as onboarded in cache (already onboarded)", projectID)
			}
		}
		return nil
	}

	// Onboard user
	onboardReqPayload := map[string]interface{}{
		"tierId":                  tier["id"],
		"cloudaicompanionProject": projectID,
		"metadata":                config.GetClientMetadata(projectID),
	}

	for {
		lroData, err := makeAPIRequest(token, "/v1internal:onboardUser", onboardReqPayload)
		if err != nil {
			return fmt.Errorf("user onboarding failed: %v", err)
		}

		if done, _ := lroData["done"].(bool); done {
			// Mark project as onboarded in cache after successful onboarding
			if onboardingCache != nil {
				onboardingCache.MarkOnboarded(projectID)
				if config.IsDebugEnabled() {
					log.Printf("[DEBUG] Project %s marked as onboarded in cache (newly onboarded)", projectID)
				}
			}
			break
		}

		time.Sleep(5 * time.Second)
	}

	return nil
}

// GetUserProjectID gets the user's project ID
func GetUserProjectID(token *oauth2.Token) (string, error) {
	// Priority 1: Check environment variable
	envProjectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if envProjectID != "" {
		log.Printf("Using project ID from GOOGLE_CLOUD_PROJECT environment variable: %s", envProjectID)
		userProjectID = envProjectID
		SaveCredentials(token, userProjectID)
		return userProjectID, nil
	}

	// If we already have a cached project_id, use it
	if userProjectID != "" {
		log.Printf("Using cached project ID: %s", userProjectID)
		return userProjectID, nil
	}

	// Priority 2: Check cached project ID in credential file
	if fileExists(config.CredentialFile) {
		data, err := os.ReadFile(config.CredentialFile)
		if err == nil {
			var credsData map[string]interface{}
			if err := json.Unmarshal(data, &credsData); err == nil {
				if cachedProjectID, ok := credsData["project_id"].(string); ok && cachedProjectID != "" {
					log.Printf("Using cached project ID from credential file: %s", cachedProjectID)
					userProjectID = cachedProjectID
					return userProjectID, nil
				}
			}
		}
	}

	// Priority 3: Make API call to discover project ID
	if token.Expiry.Before(time.Now()) && token.RefreshToken != "" {
		log.Println("Refreshing credentials before project ID discovery...")
		newToken, err := oauthConfig.TokenSource(context.Background(), token).Token()
		if err != nil {
			log.Printf("Failed to refresh credentials while getting project ID: %v", err)
		} else {
			token = newToken
			SaveCredentials(token, "")
			log.Println("Credentials refreshed successfully for project ID discovery")
		}
	}

	if token.AccessToken == "" {
		return "", errors.New("no valid access token available for project ID discovery")
	}

	probePayload := map[string]interface{}{
		"metadata": config.GetClientMetadata(""),
	}

	log.Println("Attempting to discover project ID via API call...")
	data, err := makeAPIRequest(token, "/v1internal:loadCodeAssist", probePayload)
	if err != nil {
		return "", fmt.Errorf("failed to discover project ID via API: %v", err)
	}

	discoveredProjectID, ok := data["cloudaicompanionProject"].(string)
	if !ok || discoveredProjectID == "" {
		return "", errors.New("could not find 'cloudaicompanionProject' in loadCodeAssist response")
	}

	log.Printf("Discovered project ID via API: %s", discoveredProjectID)
	userProjectID = discoveredProjectID
	SaveCredentials(token, userProjectID)

	return userProjectID, nil
}

func makeAPIRequest(token *oauth2.Token, endpoint string, payload map[string]interface{}) (map[string]interface{}, error) {
	// Use dynamic endpoint getter to support runtime configuration changes
	apiEndpoint := config.GetCodeAssistEndpoint()
	url := apiEndpoint + endpoint

	log.Printf("[DEBUG] makeAPIRequest - Endpoint: %s, Path: %s, Full URL: %s", apiEndpoint, endpoint, url)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", config.GetUserAgent())

	// Use the shared HTTP client for connection pooling
	resp, err := httputil.SharedHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// InitializeCredentialPool initializes the global credential pool by loading credentials
// from the configured folder and legacy sources for backward compatibility
func InitializeCredentialPool() error {
	log.Printf("[INFO] Initializing credential pool...")

	// Initialize the onboarding cache
	onboardingCache = NewOnboardingCache()
	log.Printf("[INFO] Onboarding cache initialized")

	// Create new credential pool
	credentialPool = NewCredentialPool()

	// Create rate-limited pool with configured RPS limit
	credentialRateLimitRPS := config.GetCredentialRateLimitRPS()
	minInterval := time.Second / time.Duration(credentialRateLimitRPS)
	rateLimitedPool = NewRateLimitedCredentialPool(minInterval)
	rateLimitedPool.CredentialPool = credentialPool

	if config.IsRateLimitingEnabled() {
		log.Printf("[INFO] Rate-limited credential pool initialized (max %d RPS per credential, min interval: %v)",
			credentialRateLimitRPS, minInterval)
	} else {
		log.Printf("[WARN] Rate limiting is DISABLED - may cause 429 errors at high RPS")
	}

	// Get credentials folder path from config
	credsFolder := config.OAuthCredsFolder

	// Create credentials folder if it doesn't exist
	if err := os.MkdirAll(credsFolder, 0700); err != nil {
		log.Printf("[WARN] Failed to create credentials folder %s: %v", credsFolder, err)
	}

	// Log the credentials folder path being used
	log.Printf("[INFO] Using credentials folder: %s", credsFolder)

	// Track initial pool size to determine if folder is empty
	initialSize := credentialPool.Size()
	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Initial pool size: %d", initialSize)
	}

	// Load credentials from folder
	if err := LoadCredentialsFromFolder(credsFolder, credentialPool); err != nil {
		log.Printf("[WARN] Failed to load credentials from folder: %v", err)
	}

	// Determine if folder was empty (no credentials loaded from folder)
	folderIsEmpty := (credentialPool.Size() == initialSize)
	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Folder is empty: %v (pool size after folder load: %d)", folderIsEmpty, credentialPool.Size())
	}

	// Load legacy credentials for backward compatibility
	legacyCount := LoadLegacyCredential(credentialPool, config.ScriptDir, folderIsEmpty)
	if legacyCount > 0 {
		log.Printf("[INFO] Loaded %d legacy credential(s) for backward compatibility", legacyCount)
	}

	// Log final credential count
	totalCredentials := credentialPool.Size()
	if totalCredentials == 0 {
		log.Printf("[WARN] Credential pool initialized with 0 credentials - API requests will fail until credentials are added")
	} else {
		log.Printf("[INFO] Credential pool initialized with %d credential(s)", totalCredentials)
	}

	// Return nil to allow server to start even with no credentials
	// API requests will fail with appropriate error messages if no credentials available
	return nil
}

// GetCredentialForRequest selects a credential from the pool for an API request
// Uses rate-limited round-robin selection if enabled, otherwise random selection
func GetCredentialForRequest() (*CredentialEntry, error) {
	// Check if credential pool is initialized
	if credentialPool == nil {
		return nil, errors.New("credential pool not initialized")
	}

	var credEntry *CredentialEntry
	var err error

	// Use rate-limited pool if enabled
	if config.IsRateLimitingEnabled() && rateLimitedPool != nil {
		credEntry, err = rateLimitedPool.GetCredentialWithRateLimit()
		if err != nil {
			return nil, err
		}
	} else {
		// Fallback to random selection
		credEntry, err = credentialPool.GetRandomCredential()
		if err != nil {
			return nil, err
		}
	}

	// Log selected credential's project ID at debug level
	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Selected credential with project ID: %s", credEntry.ProjectID)
	}

	return credEntry, nil
}

// ResetOnboardingState clears the onboarding cache
func ResetOnboardingState() {
	if onboardingCache != nil {
		onboardingCache.Clear()
		if config.IsDebugEnabled() {
			log.Printf("[DEBUG] Onboarding cache cleared")
		}
	}
}

// ReloadCredentialPool reloads the credential pool from disk
// This should be called after credentials are added or removed via the dashboard
func ReloadCredentialPool() error {
	log.Printf("[INFO] Reloading credential pool...")

	// Create new credential pool
	newPool := NewCredentialPool()

	// Get credentials folder path from config
	credsFolder := config.OAuthCredsFolder

	// Load credentials from folder
	if err := LoadCredentialsFromFolder(credsFolder, newPool); err != nil {
		log.Printf("[WARN] Failed to load credentials from folder during reload: %v", err)
	}

	// Load legacy credentials for backward compatibility
	legacyCount := LoadLegacyCredential(newPool, config.ScriptDir, newPool.Size() == 0)
	if legacyCount > 0 {
		log.Printf("[INFO] Loaded %d legacy credential(s) during reload", legacyCount)
	}

	// Replace the global credential pool
	credentialPool = newPool

	// Log final credential count
	totalCredentials := credentialPool.Size()
	if totalCredentials == 0 {
		log.Printf("[WARN] Credential pool reloaded with 0 credentials")
	} else {
		log.Printf("[INFO] Credential pool reloaded with %d credential(s)", totalCredentials)
	}

	return nil
}

// GetCredentialPoolSize returns the number of available (unbanned) credentials in the pool
func GetCredentialPoolSize() int {
	if credentialPool == nil {
		return 0
	}
	return credentialPool.GetAvailableCredentialCount()
}
