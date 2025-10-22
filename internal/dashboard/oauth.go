package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/client"
	"gcli2apigo/internal/config"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// tokenStore temporarily stores OAuth tokens during the streaming process
var tokenStore sync.Map

// OAuthState represents a CSRF protection state with expiration
type OAuthState struct {
	State     string
	CreatedAt time.Time
}

// OAuthHandler manages the OAuth flow for credential creation
// Security features:
// - CSRF protection using cryptographically random state parameters (UUID v4)
// - State expiration after 10 minutes
// - Background cleanup of expired states every 5 minutes
// - Single-use state tokens (deleted after validation)
// - Thread-safe operations using RWMutex
type OAuthHandler struct {
	config     *oauth2.Config
	stateStore map[string]*OAuthState
	mu         sync.RWMutex
}

// NewOAuthHandler creates a new OAuthHandler instance
func NewOAuthHandler() *OAuthHandler {
	// Configure OAuth with scopes for Cloud Resource Manager and Service Usage APIs
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  "", // Will be set dynamically based on request
		Scopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	oh := &OAuthHandler{
		config:     oauthConfig,
		stateStore: make(map[string]*OAuthState),
	}

	// Start automatic cleanup goroutine for expired states
	go oh.cleanupLoop()

	return oh
}

// StartOAuthFlow initiates the OAuth flow by redirecting to Google's consent screen
func (oh *OAuthHandler) StartOAuthFlow(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] OAuth flow initiated from %s", r.RemoteAddr)

	// Generate CSRF protection state using UUID
	state := uuid.New().String()

	// Store state with expiration (10 minutes)
	oh.mu.Lock()
	oh.stateStore[state] = &OAuthState{
		State:     state,
		CreatedAt: time.Now(),
	}
	oh.mu.Unlock()

	log.Printf("[DEBUG] Generated OAuth state: %s (expires in 10 minutes)", state)

	// Set redirect URL dynamically based on the request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check for X-Forwarded-Proto header (common in reverse proxy setups)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	redirectURL := scheme + "://" + r.Host + "/dashboard/oauth/callback"
	oh.config.RedirectURL = redirectURL

	log.Printf("[INFO] OAuth redirect URL configured: %s", redirectURL)

	// Generate authorization URL with state parameter
	authURL := oh.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	log.Printf("[INFO] Redirecting user to Google OAuth consent screen")
	// Redirect user to Google's OAuth consent screen
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// ValidateState checks if a state parameter is valid and not expired
func (oh *OAuthHandler) ValidateState(state string) bool {
	if state == "" {
		log.Printf("[WARN] OAuth state validation failed: empty state")
		return false
	}

	oh.mu.RLock()
	oauthState, exists := oh.stateStore[state]
	oh.mu.RUnlock()

	if !exists {
		log.Printf("[WARN] OAuth state validation failed: state not found: %s", state)
		return false
	}

	// Check if state has expired (10 minutes)
	if time.Since(oauthState.CreatedAt) > 10*time.Minute {
		log.Printf("[WARN] OAuth state validation failed: state expired: %s (age: %v)", state, time.Since(oauthState.CreatedAt))
		oh.DeleteState(state)
		return false
	}

	log.Printf("[DEBUG] OAuth state validated successfully: %s", state)
	return true
}

// DeleteState removes a state from the store
func (oh *OAuthHandler) DeleteState(state string) {
	oh.mu.Lock()
	delete(oh.stateStore, state)
	oh.mu.Unlock()
	log.Printf("[DEBUG] OAuth state deleted: %s", state)
}

// CleanupExpiredStates removes all expired states (older than 10 minutes)
func (oh *OAuthHandler) CleanupExpiredStates() {
	now := time.Now()
	expirationDuration := 10 * time.Minute

	oh.mu.Lock()
	defer oh.mu.Unlock()

	cleanedCount := 0
	for state, oauthState := range oh.stateStore {
		if now.Sub(oauthState.CreatedAt) > expirationDuration {
			delete(oh.stateStore, state)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		log.Printf("[INFO] Cleaned up %d expired OAuth states", cleanedCount)
	}
}

// cleanupLoop runs a background goroutine that periodically cleans up expired states
func (oh *OAuthHandler) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		oh.CleanupExpiredStates()
	}
}

// HandleCallback processes the OAuth callback and exchanges the authorization code for tokens
func (oh *OAuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] OAuth callback received from %s", r.RemoteAddr)

	// Extract state and code from query parameters
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	// Handle authorization denied scenario
	if errorParam != "" {
		log.Printf("[WARN] OAuth authorization denied: %s", errorParam)
		errorDescription := r.URL.Query().Get("error_description")
		if errorDescription == "" {
			errorDescription = "Authorization was denied"
		}
		log.Printf("[ERROR] OAuth error details: %s - %s", errorParam, errorDescription)
		RenderOAuthCallback(w, "error", "Authorization Denied: "+errorDescription)
		return
	}

	// Validate required parameters
	if state == "" || code == "" {
		log.Printf("[ERROR] Missing required OAuth callback parameters: state=%v, code=%v", state != "", code != "")
		RenderOAuthCallback(w, "error", "Invalid callback parameters. Please try the OAuth flow again.")
		return
	}

	log.Printf("[DEBUG] OAuth callback parameters validated")

	// Validate state to prevent CSRF attacks
	if !oh.ValidateState(state) {
		log.Printf("[ERROR] Invalid or expired OAuth state: %s", state)
		RenderOAuthCallback(w, "error", "Invalid or expired session. Please try the OAuth flow again.")
		return
	}

	// Delete the state after successful validation (single-use)
	oh.DeleteState(state)

	log.Printf("[INFO] OAuth state validated successfully, exchanging authorization code for tokens...")

	// Set redirect URL dynamically (same as in StartOAuthFlow)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	redirectURL := scheme + "://" + r.Host + "/dashboard/oauth/callback"
	oh.config.RedirectURL = redirectURL

	// Exchange authorization code for tokens
	ctx := context.Background()
	token, err := oh.config.Exchange(ctx, code)
	if err != nil {
		log.Printf("[ERROR] Failed to exchange authorization code: %v", err)
		RenderOAuthCallback(w, "error", "Failed to exchange authorization code. Please try again or check your OAuth configuration.")
		return
	}

	log.Printf("[INFO] Successfully exchanged authorization code for tokens (expires: %v)", token.Expiry)

	// Store token temporarily for the streaming process
	sessionID := uuid.New().String()
	oh.mu.Lock()
	oh.stateStore[sessionID] = &OAuthState{
		State:     sessionID,
		CreatedAt: time.Now(),
	}
	oh.mu.Unlock()

	// Store the token in a temporary map (will be cleaned up after processing)
	tokenStore.Store(sessionID, token)

	// Redirect to streaming endpoint
	streamURL := scheme + "://" + r.Host + "/dashboard/oauth/process?session=" + sessionID
	http.Redirect(w, r, streamURL, http.StatusSeeOther)
}

// HandleOAuthProcess handles the streaming OAuth processing
func (oh *OAuthHandler) HandleOAuthProcess(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		log.Printf("[ERROR] Missing session parameter")
		RenderOAuthCallback(w, "error", "Invalid session. Please try the OAuth flow again.")
		return
	}

	// Retrieve token from temporary store
	tokenInterface, ok := tokenStore.Load(sessionID)
	if !ok {
		log.Printf("[ERROR] Session not found: %s", sessionID)
		RenderOAuthCallback(w, "error", "Session expired or invalid. Please try the OAuth flow again.")
		return
	}

	token, ok := tokenInterface.(*oauth2.Token)
	if !ok {
		log.Printf("[ERROR] Invalid token type in session: %s", sessionID)
		RenderOAuthCallback(w, "error", "Internal error. Please try the OAuth flow again.")
		return
	}

	// Check if this is the initial page load or SSE connection
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "text/event-stream") {
		// Initial page load - render the streaming template
		// Don't clean up yet - SSE connection will come next
		RenderOAuthCallbackStream(w)
		return
	}

	// Clean up token from store after SSE processing completes
	defer tokenStore.Delete(sessionID)
	defer oh.DeleteState(sessionID)

	// Set up Server-Sent Events for streaming progress
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable buffering in nginx

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("[ERROR] Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Helper function to send SSE events
	sendEvent := func(eventType, message string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, message)
		flusher.Flush()
	}

	// Send initial progress event
	sendEvent("progress", "Authorization successful! Starting project discovery...")

	// Discover all projects in the account
	log.Printf("[INFO] Starting project discovery...")
	gcpClient := NewGCPClient(token)
	projects, err := gcpClient.ListProjects()
	if err != nil {
		log.Printf("[ERROR] Failed to discover projects: %v", err)
		sendEvent("error", fmt.Sprintf("Failed to discover projects: %v", err))
		sendEvent("done", "error")
		return
	}

	if len(projects) == 0 {
		log.Printf("[WARN] No active projects found in the account")
		sendEvent("warning", "No active projects found in your account. Please create a project first.")
		sendEvent("done", "warning")
		return
	}

	log.Printf("[INFO] Discovered %d active projects", len(projects))
	sendEvent("progress", fmt.Sprintf("Discovered %d active project(s). Processing...", len(projects)))

	// Process each project: enable APIs and save credentials
	successCount := 0
	credentialFailures := 0
	apiWarnings := []string{}

	for i, project := range projects {
		log.Printf("[INFO] Processing project %d/%d: %s (%s)", i+1, len(projects), project.ProjectID, project.ProjectName)
		sendEvent("progress", fmt.Sprintf("Processing project %d/%d: %s", i+1, len(projects), project.ProjectID))

		// Enable Gemini APIs for the project (treat failures as warnings)
		apiResults := gcpClient.EnableGeminiAPIs(project.ProjectID)

		// Check if any API enablement failed (but don't fail the whole process)
		for serviceName, err := range apiResults {
			if err != nil {
				warningMsg := fmt.Sprintf("Warning: Failed to enable %s for project %s", serviceName, project.ProjectID)
				log.Printf("[WARN] %s: %v", warningMsg, err)
				apiWarnings = append(apiWarnings, warningMsg)
				sendEvent("warning", warningMsg)
			} else {
				log.Printf("[INFO] Successfully enabled %s for project %s", serviceName, project.ProjectID)
			}
		}

		// Save credential file for the project
		if err := oh.saveCredential(token, project.ProjectID); err != nil {
			log.Printf("[ERROR] Failed to save credential for project %s: %v", project.ProjectID, err)
			sendEvent("error", fmt.Sprintf("Failed to save credential for project %s: %v", project.ProjectID, err))
			credentialFailures++
			continue
		}

		log.Printf("[INFO] Successfully saved credential for project: %s", project.ProjectID)
		sendEvent("success", fmt.Sprintf("✓ Saved credential for project: %s", project.ProjectID))
		successCount++
	}

	// Reload credential pool after saving credentials
	if successCount > 0 {
		if err := auth.ReloadCredentialPool(); err != nil {
			log.Printf("[WARN] Failed to reload credential pool after OAuth flow: %v", err)
			sendEvent("warning", "Credentials saved but pool reload failed. Server restart may be required.")
		} else {
			log.Printf("[INFO] Credential pool reloaded successfully with %d new credential(s)", successCount)
		}
	}

	// Send final summary
	if successCount > 0 && credentialFailures == 0 {
		summaryMsg := fmt.Sprintf("Successfully processed %d project(s)!", successCount)
		if len(apiWarnings) > 0 {
			summaryMsg += fmt.Sprintf(" Note: %d API enablement warning(s) occurred.", len(apiWarnings))
		}
		log.Printf("[INFO] OAuth flow completed: %s", summaryMsg)
		sendEvent("complete", summaryMsg)
		sendEvent("done", "success")
	} else if successCount > 0 && credentialFailures > 0 {
		summaryMsg := fmt.Sprintf("Partially successful: %d project(s) saved, %d failed.", successCount, credentialFailures)
		if len(apiWarnings) > 0 {
			summaryMsg += fmt.Sprintf(" Also, %d API enablement warning(s) occurred.", len(apiWarnings))
		}
		log.Printf("[WARN] OAuth flow partially successful: %s", summaryMsg)
		sendEvent("complete", summaryMsg)
		sendEvent("done", "warning")
	} else {
		summaryMsg := fmt.Sprintf("Failed to save credentials for all projects. %d project(s) failed.", credentialFailures)
		log.Printf("[ERROR] OAuth flow failed: %s", summaryMsg)
		sendEvent("complete", summaryMsg)
		sendEvent("done", "error")
	}
}

// ExchangeCode exchanges an authorization code for OAuth tokens
// This is a helper method that will be used in the callback handler (task 4)
func (oh *OAuthHandler) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return oh.config.Exchange(ctx, code)
}

// GetConfig returns the OAuth configuration
// This can be useful for testing or other components
func (oh *OAuthHandler) GetConfig() *oauth2.Config {
	return oh.config
}

// NewGCPClient creates a new GCP client with the provided OAuth token
// This is a convenience wrapper around client.NewGCPClient
func NewGCPClient(token *oauth2.Token) *client.GCPClient {
	return client.NewGCPClient(token)
}

// saveCredential saves OAuth tokens to a credential file in the oauth_creds directory
// The file is named {project_id}.json and contains all necessary OAuth information
func (oh *OAuthHandler) saveCredential(token *oauth2.Token, projectID string) error {
	log.Printf("[INFO] Saving credential for project: %s", projectID)

	// Validate project_id to prevent path traversal and injection attacks
	if err := ValidateProjectID(projectID); err != nil {
		log.Printf("[ERROR] Save credential failed: invalid project_id: %v", err)
		return fmt.Errorf("invalid project_id: %w", err)
	}

	// Ensure oauth_creds directory exists with 0700 permissions (owner read/write/execute only)
	credsDir := config.OAuthCredsFolder
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		log.Printf("[ERROR] Failed to create oauth_creds directory %s: %v", credsDir, err)
		return fmt.Errorf("failed to create oauth_creds directory: %w", err)
	}

	// Verify directory permissions are correct (0700)
	dirInfo, err := os.Stat(credsDir)
	if err != nil {
		log.Printf("[ERROR] Failed to stat credentials directory: %v", err)
		return fmt.Errorf("failed to stat credentials directory: %w", err)
	}

	// Check if permissions are correct (0700 = drwx------)
	if dirInfo.Mode().Perm() != 0700 {
		log.Printf("[WARN] Credentials directory has incorrect permissions: %o, fixing to 0700", dirInfo.Mode().Perm())
		if err := os.Chmod(credsDir, 0700); err != nil {
			log.Printf("[ERROR] Failed to fix directory permissions: %v", err)
			return fmt.Errorf("failed to fix directory permissions: %w", err)
		}
	}

	log.Printf("[DEBUG] Credential directory verified: %s (permissions: 0700)", credsDir)

	// Extract client credentials from token extra data or use defaults
	clientID := config.ClientID
	clientSecret := config.ClientSecret
	tokenURI := "https://oauth2.googleapis.com/token"

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
	if extra := token.Extra("token_uri"); extra != nil {
		if uri, ok := extra.(string); ok && uri != "" {
			tokenURI = uri
		}
	}

	// Create credential data structure matching existing format
	credData := map[string]interface{}{
		"access_token":  token.AccessToken,
		"client_id":     clientID,
		"client_secret": clientSecret,
		"expiry":        token.Expiry.Format(time.RFC3339),
		"project_id":    projectID,
		"refresh_token": token.RefreshToken,
		"token_type":    token.TokenType,
		"token_uri":     tokenURI,
	}

	// Marshal to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(credData, "", "  ")
	if err != nil {
		log.Printf("[ERROR] Failed to marshal credential data for project %s: %v", projectID, err)
		return fmt.Errorf("failed to marshal credential data: %w", err)
	}

	// Create file path: {project_id}.json
	filePath := filepath.Join(credsDir, projectID+".json")

	// Additional security check: Ensure the resolved path is still within the credentials directory
	absCredsDir, err := filepath.Abs(credsDir)
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
		log.Printf("[ERROR] Security violation: attempted to write file outside credentials directory: %s", absFilePath)
		return fmt.Errorf("security violation: file path outside credentials directory")
	}

	log.Printf("[DEBUG] Writing credential file: %s", filePath)

	// Write credential file with 0600 permissions (owner read/write only)
	// This will overwrite existing files with the same project ID
	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		log.Printf("[ERROR] Failed to write credential file %s: %v", filePath, err)
		return fmt.Errorf("failed to write credential file: %w", err)
	}

	// Verify file permissions are correct (0600)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("[ERROR] Failed to stat credential file: %v", err)
		return fmt.Errorf("failed to stat credential file: %w", err)
	}

	// Check if permissions are correct (0600 = -rw-------)
	if fileInfo.Mode().Perm() != 0600 {
		log.Printf("[WARN] Credential file has incorrect permissions: %o, fixing to 0600", fileInfo.Mode().Perm())
		if err := os.Chmod(filePath, 0600); err != nil {
			log.Printf("[ERROR] Failed to fix file permissions: %v", err)
			return fmt.Errorf("failed to fix file permissions: %w", err)
		}
	}

	log.Printf("[INFO] Successfully saved credential to: %s (permissions: 0600)", filePath)
	return nil
}
