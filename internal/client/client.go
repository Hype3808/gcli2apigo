package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/httputil"
	"gcli2apigo/internal/usage"

	"golang.org/x/oauth2"
)

// TokenRefreshManager manages token refresh operations with per-credential locking
// to ensure only one refresh happens per credential even with concurrent requests
type TokenRefreshManager struct {
	// refreshMutexes stores a mutex for each credential file path
	// Using sync.Map for thread-safe concurrent access without global lock
	refreshMutexes sync.Map // map[string]*sync.Mutex
}

// NewTokenRefreshManager creates a new TokenRefreshManager instance
func NewTokenRefreshManager() *TokenRefreshManager {
	return &TokenRefreshManager{}
}

// RefreshToken refreshes the OAuth token for a credential with per-credential locking
// This ensures only one refresh operation happens per credential even with concurrent requests
func (trm *TokenRefreshManager) RefreshToken(credEntry *auth.CredentialEntry) error {
	if credEntry == nil || credEntry.FilePath == "" {
		return fmt.Errorf("invalid credential entry")
	}

	// Get or create a mutex for this specific credential
	mutexInterface, _ := trm.refreshMutexes.LoadOrStore(credEntry.FilePath, &sync.Mutex{})
	mutex := mutexInterface.(*sync.Mutex)

	// Acquire the credential-specific lock
	mutex.Lock()
	defer mutex.Unlock()

	// Check if token still needs refresh (another goroutine may have already refreshed it)
	if !credEntry.Token.Expiry.Before(time.Now()) && credEntry.Token.AccessToken != "" {
		if config.IsDebugEnabled() {
			log.Printf("[DEBUG] Token already refreshed by another request for credential: %s", credEntry.FilePath)
		}
		return nil
	}

	// Perform the actual token refresh
	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Refreshing token for credential: %s (expiry: %s)", credEntry.FilePath, credEntry.Token.Expiry.Format(time.RFC3339))
	}

	// Extract client credentials from token extra data or use defaults
	clientID := config.ClientID
	clientSecret := config.ClientSecret
	if extra := credEntry.Token.Extra("client_id"); extra != nil {
		if id, ok := extra.(string); ok && id != "" {
			clientID = id
		}
	}
	if extra := credEntry.Token.Extra("client_secret"); extra != nil {
		if secret, ok := extra.(string); ok && secret != "" {
			clientSecret = secret
		}
	}

	oauthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: config.OAuth2Endpoint + "/token",
		},
	}

	tokenSource := oauthConfig.TokenSource(context.Background(), credEntry.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Printf("[WARN] Token refresh failed for credential %s: %v", credEntry.FilePath, err)
		return fmt.Errorf("token refresh failed: %v", err)
	}

	if config.IsDebugEnabled() {
		log.Printf("[DEBUG] Token refreshed successfully for credential: %s (new expiry: %s)", credEntry.FilePath, newToken.Expiry.Format(time.RFC3339))
	}

	// Update the credential entry with the new token
	credEntry.Token = newToken

	// Save the refreshed token asynchronously to avoid blocking
	auth.SaveRefreshedTokenAsync(credEntry)

	return nil
}

var (
	// globalTokenRefreshManager is the global instance used across the application
	globalTokenRefreshManager = NewTokenRefreshManager()
)

// SendGeminiRequest sends a request to Google's Gemini API
// Process: 1. Randomly obtain OAuth credential, 2. Refresh token if needed, 3. Make API request, 4. Return
// If a 429 error occurs, automatically retry with different OAuth credentials until success or all credentials exhausted
func SendGeminiRequest(payload map[string]any, isStreaming bool) (any, error) {
	// Track which credentials have been tried to avoid retrying the same one
	triedCredentials := make(map[string]bool)

	// Extract model name for usage tracking
	modelName := ""
	if model, ok := payload["model"].(string); ok {
		modelName = model
	}

	// Retry loop: try different credentials on 429 errors
	// Limited by MAX_RETRY_ATTEMPTS (default: 5)
	// This is read dynamically to allow runtime updates without restart
	maxRetries := config.GetMaxRetryAttempts()
	if maxRetries <= 0 {
		maxRetries = 5 // Ensure at least 1 attempt
	}

	for {
		// Step 1: Randomly obtain an OAuth credential from the oauth_creds folder
		credEntry, err := auth.GetCredentialForRequest()
		if err != nil {
			log.Printf("[ERROR] Credential selection failed: %v", err)
			return nil, fmt.Errorf("credential selection failed: %v", err)
		}

		// Check if we've already tried this credential
		if triedCredentials[credEntry.ProjectID] {
			// Check if we've reached max retry attempts or tried all available credentials
			poolSize := auth.GetCredentialPoolSize()
			if len(triedCredentials) >= maxRetries || len(triedCredentials) >= poolSize {
				log.Printf("[ERROR] Retry limit reached: tried %d credentials (max: %d, pool size: %d)",
					len(triedCredentials), maxRetries, poolSize)
				return nil, fmt.Errorf("rate limit exceeded: retry limit reached after %d attempts", len(triedCredentials))
			}
			// Skip this credential and try to get another one
			continue
		}

		// Mark this credential as tried
		triedCredentials[credEntry.ProjectID] = true

		creds := credEntry.Token
		projID := credEntry.ProjectID
		if config.IsDebugEnabled() {
			log.Printf("[DEBUG] Selected credential from: %s (project: %s) [attempt %d/%d]",
				credEntry.FilePath, projID, len(triedCredentials), auth.GetCredentialPoolSize())
		}

		// Step 2: Refresh the token if needed (expired OR no access token)
		needsRefresh := creds.Expiry.Before(time.Now()) || creds.AccessToken == ""

		if needsRefresh && creds.RefreshToken != "" {
			if config.IsDebugEnabled() {
				if creds.AccessToken == "" {
					log.Printf("[DEBUG] No access token, refreshing for credential: %s", credEntry.FilePath)
				} else {
					log.Printf("[DEBUG] Token expired (expiry: %s), refreshing for credential: %s", creds.Expiry.Format(time.RFC3339), credEntry.FilePath)
				}
			}

			// Use TokenRefreshManager to handle refresh with per-credential locking
			err := globalTokenRefreshManager.RefreshToken(credEntry)
			if err != nil {
				log.Printf("Warning: Token refresh failed for credential %s: %v", credEntry.FilePath, err)
				if creds.AccessToken == "" {
					// Try next credential
					continue
				}
				// Continue with existing token as per requirement 2.4
			} else {
				// Token was refreshed successfully, update local reference
				creds = credEntry.Token
			}
		} else if creds.AccessToken == "" {
			log.Printf("[WARN] No access token available for credential %s, trying next credential", credEntry.FilePath)
			continue
		} else {
			if config.IsDebugEnabled() {
				log.Printf("[DEBUG] Token is still valid (expiry: %s)", creds.Expiry.Format(time.RFC3339))
			}
		}

		// Step 3: Make API request (onboarding and actual request)

		// Onboard user with selected credential
		err = auth.OnboardUser(creds, projID)
		if err != nil {
			// Check if it's a 401 error and try refreshing the token
			if strings.Contains(err.Error(), "401") && creds.RefreshToken != "" {
				if config.IsDebugEnabled() {
					log.Printf("[DEBUG] Got 401 during onboarding, forcing token refresh...")
				}

				// Reset onboarding state since credentials are invalid
				auth.ResetOnboardingState()

				// Use TokenRefreshManager to handle refresh with per-credential locking
				refreshErr := globalTokenRefreshManager.RefreshToken(credEntry)
				if refreshErr != nil {
					log.Printf("Warning: Failed to refresh token after 401: %v", refreshErr)
					// Try next credential
					continue
				}

				if config.IsDebugEnabled() {
					log.Printf("[DEBUG] Token refreshed after 401, retrying onboarding...")
				}
				// Update local reference to refreshed token
				creds = credEntry.Token

				// Retry onboarding with refreshed token
				if retryErr := auth.OnboardUser(creds, projID); retryErr != nil {
					log.Printf("[WARN] Failed to onboard user after token refresh: %v, trying next credential", retryErr)
					continue
				}
				if config.IsDebugEnabled() {
					log.Printf("[DEBUG] Onboarding successful after token refresh")
				}
			} else {
				log.Printf("[WARN] Failed to onboard user: %v, trying next credential", err)
				continue
			}
		}

		// Build the final payload with project info
		requestData, _ := payload["request"].(map[string]any)
		if requestData == nil {
			requestData = make(map[string]any)
		}

		finalPayload := map[string]any{
			"model":   payload["model"],
			"project": projID,
			"request": requestData,
		}

		// Determine the action and URL using strings.Builder to avoid allocations
		action := "generateContent"
		if isStreaming {
			action = "streamGenerateContent"
		}

		var urlBuilder strings.Builder
		urlBuilder.WriteString(config.CodeAssistEndpoint)
		urlBuilder.WriteString("/v1internal:")
		urlBuilder.WriteString(action)
		if isStreaming {
			urlBuilder.WriteString("?alt=sse")
		}
		targetURL := urlBuilder.String()

		// Build request
		jsonData, err := json.Marshal(finalPayload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		// Use strings.Builder to avoid string allocation in hot path
		var authHeader strings.Builder
		authHeader.WriteString("Bearer ")
		authHeader.WriteString(creds.AccessToken)
		req.Header.Set("Authorization", authHeader.String())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", config.GetUserAgent())

		// Send the request using the shared HTTP client
		resp, err := httputil.SharedHTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %v", err)
		}

		// Check for 429 error and retry with different credential
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			log.Printf("[WARN] Received 429 (Too Many Requests) for project %s, retrying with different credential... (attempt %d/%d)",
				projID, len(triedCredentials), maxRetries)

			// Track error code for this project
			usage.GetTracker().SetErrorCode(projID, resp.StatusCode)

			// Check if we've reached max retry attempts or tried all credentials
			poolSize := auth.GetCredentialPoolSize()
			if len(triedCredentials) >= maxRetries || len(triedCredentials) >= poolSize {
				log.Printf("[ERROR] Retry limit reached: tried %d credentials (max: %d, pool size: %d)",
					len(triedCredentials), maxRetries, poolSize)
				return nil, fmt.Errorf("rate limit exceeded: retry limit reached after %d attempts", len(triedCredentials))
			}

			// Continue to next iteration to try another credential
			continue
		}

		// Step 4: Return response
		var result any
		var responseErr error

		if isStreaming {
			result, responseErr = handleStreamingResponse(resp)
		} else {
			result, responseErr = handleNonStreamingResponse(resp)
		}

		// Track usage and error status
		if responseErr == nil && resp.StatusCode == http.StatusOK {
			isProModel := usage.IsProModel(modelName)
			usage.GetTracker().IncrementUsage(projID, isProModel)
			if config.IsDebugEnabled() {
				log.Printf("[DEBUG] Usage tracked for project %s (model: %s, isPro: %v)", projID, modelName, isProModel)
			}
		} else if resp.StatusCode != http.StatusOK {
			// Track error code for this project
			usage.GetTracker().SetErrorCode(projID, resp.StatusCode)
			if config.IsDebugEnabled() {
				log.Printf("[DEBUG] Error code %d tracked for project %s", resp.StatusCode, projID)
			}
		}

		return result, responseErr
	}
}

func handleStreamingResponse(resp *http.Response) (chan string, error) {
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Google API returned status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	streamChan := make(chan string, 100)

	go func() {
		defer close(streamChan)
		defer resp.Body.Close()

		// Use larger buffer for scanner to handle large chunks
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 64*1024)
		scanner.Buffer(buf, 256*1024)

		for scanner.Scan() {
			line := scanner.Text()
			// Use CutPrefix to avoid double prefix check and allocation
			if chunk, found := strings.CutPrefix(line, "data: "); found {

				var obj map[string]any
				if err := json.Unmarshal([]byte(chunk), &obj); err != nil {
					continue
				}

				if response, ok := obj["response"].(map[string]any); ok {
					responseJSON, _ := json.Marshal(response)
					streamChan <- string(responseJSON)
				} else {
					streamChan <- chunk
				}
			}
		}
	}()

	return streamChan, nil
}

func handleNonStreamingResponse(resp *http.Response) (map[string]any, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google API returned status %d: %s", resp.StatusCode, string(body))

		var errorData map[string]any
		if err := json.Unmarshal(body, &errorData); err == nil {
			if errObj, ok := errorData["error"].(map[string]any); ok {
				return map[string]any{
					"error": errObj,
				}, nil
			}
		}

		return map[string]any{
			"error": map[string]any{
				"message": fmt.Sprintf("API error: %d", resp.StatusCode),
				"code":    resp.StatusCode,
			},
		}, nil
	}

	// Parse response - use CutPrefix to avoid allocation and double check
	responseText, _ := strings.CutPrefix(string(body), "data: ")

	var googleAPIResponse map[string]any
	if err := json.Unmarshal([]byte(responseText), &googleAPIResponse); err != nil {
		return nil, err
	}

	if standardGeminiResponse, ok := googleAPIResponse["response"].(map[string]any); ok {
		return standardGeminiResponse, nil
	}

	return googleAPIResponse, nil
}

// BuildGeminiPayloadFromOpenAI builds a Gemini API payload from an OpenAI-transformed request
func BuildGeminiPayloadFromOpenAI(openaiPayload map[string]any) map[string]any {
	model, _ := openaiPayload["model"].(string)

	safetySettings := config.DefaultSafetySettings
	if ss, ok := openaiPayload["safetySettings"]; ok && ss != nil {
		if ssSlice, ok := ss.([]config.SafetySetting); ok {
			safetySettings = ssSlice
		}
	}

	requestData := map[string]any{
		"contents":         openaiPayload["contents"],
		"safetySettings":   safetySettings,
		"generationConfig": openaiPayload["generationConfig"],
	}

	if systemInstruction, ok := openaiPayload["systemInstruction"]; ok && systemInstruction != nil {
		requestData["systemInstruction"] = systemInstruction
	}
	if cachedContent, ok := openaiPayload["cachedContent"]; ok && cachedContent != nil {
		requestData["cachedContent"] = cachedContent
	}
	if tools, ok := openaiPayload["tools"]; ok && tools != nil {
		requestData["tools"] = tools
	}
	if toolConfig, ok := openaiPayload["toolConfig"]; ok && toolConfig != nil {
		requestData["toolConfig"] = toolConfig
	}

	return map[string]any{
		"model":   model,
		"request": requestData,
	}
}

// BuildGeminiPayloadFromNative builds a Gemini API payload from a native Gemini request
func BuildGeminiPayloadFromNative(nativeRequest map[string]any, modelFromPath string) map[string]any {
	nativeRequest["safetySettings"] = config.DefaultSafetySettings

	// Ensure generationConfig exists
	var genConfig map[string]any
	if existingConfig, ok := nativeRequest["generationConfig"].(map[string]any); ok {
		genConfig = existingConfig
	} else {
		genConfig = make(map[string]any)
		nativeRequest["generationConfig"] = genConfig
	}

	// Set minimum thinking budget if not already specified
	if _, hasThinkingConfig := genConfig["thinkingConfig"]; !hasThinkingConfig {
		genConfig["thinkingConfig"] = map[string]any{
			"thinkingBudget": config.GetThinkingBudget(modelFromPath),
		}
	}

	return map[string]any{
		"model":   modelFromPath,
		"request": nativeRequest,
	}
}

// GeminiRequestResult holds the result of a parallel Gemini request
type GeminiRequestResult struct {
	Index    int   // Original index in the batch
	Response any   // Response data (chan string for streaming, map for non-streaming)
	Error    error // Error if request failed
}

// SendGeminiRequestsParallel sends multiple Gemini requests in parallel using goroutines
// Returns a slice of results in the same order as the input payloads
// This is useful for batch processing multiple independent requests
func SendGeminiRequestsParallel(payloads []map[string]any, isStreaming bool) []GeminiRequestResult {
	results := make([]GeminiRequestResult, len(payloads))
	var wg sync.WaitGroup

	// Use buffered channel to collect results
	resultChan := make(chan GeminiRequestResult, len(payloads))

	// Launch goroutines for each request
	for i, payload := range payloads {
		wg.Add(1)
		go func(index int, p map[string]any) {
			defer wg.Done()

			response, err := SendGeminiRequest(p, isStreaming)
			resultChan <- GeminiRequestResult{
				Index:    index,
				Response: response,
				Error:    err,
			}
		}(i, payload)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and place them in correct order
	for result := range resultChan {
		results[result.Index] = result
	}

	return results
}

// SendGeminiRequestsParallelWithLimit sends multiple Gemini requests in parallel with concurrency limit
// maxConcurrent controls how many requests can run simultaneously
// This prevents overwhelming the system or hitting rate limits too quickly
func SendGeminiRequestsParallelWithLimit(payloads []map[string]any, isStreaming bool, maxConcurrent int) []GeminiRequestResult {
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default to 10 concurrent requests
	}

	results := make([]GeminiRequestResult, len(payloads))
	var wg sync.WaitGroup

	// Semaphore to limit concurrent requests
	semaphore := make(chan struct{}, maxConcurrent)
	resultChan := make(chan GeminiRequestResult, len(payloads))

	// Launch goroutines for each request
	for i, payload := range payloads {
		wg.Add(1)
		go func(index int, p map[string]any) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			response, err := SendGeminiRequest(p, isStreaming)
			resultChan <- GeminiRequestResult{
				Index:    index,
				Response: response,
				Error:    err,
			}
		}(i, payload)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and place them in correct order
	for result := range resultChan {
		results[result.Index] = result
	}

	return results
}
