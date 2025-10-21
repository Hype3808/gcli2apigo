package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/usage"

	"golang.org/x/oauth2"
)

// SendGeminiRequest sends a request to Google's Gemini API
// Process: 1. Randomly obtain OAuth credential, 2. Refresh token if needed, 3. Make API request, 4. Return
func SendGeminiRequest(payload map[string]interface{}, isStreaming bool) (interface{}, error) {
	// Step 1: Randomly obtain an OAuth credential from the oauth_creds folder
	credEntry, err := auth.GetCredentialForRequest()
	if err != nil {
		return nil, fmt.Errorf("credential selection failed: %v", err)
	}

	creds := credEntry.Token
	projID := credEntry.ProjectID
	log.Printf("[DEBUG] Selected credential from: %s (project: %s)", credEntry.FilePath, projID)

	// Extract model name for usage tracking
	modelName := ""
	if model, ok := payload["model"].(string); ok {
		modelName = model
	}

	// Step 2: Refresh the token if needed (expired OR no access token)
	needsRefresh := creds.Expiry.Before(time.Now()) || creds.AccessToken == ""

	if needsRefresh && creds.RefreshToken != "" {
		if creds.AccessToken == "" {
			log.Printf("[DEBUG] No access token, refreshing for credential: %s", credEntry.FilePath)
		} else {
			log.Printf("[DEBUG] Token expired (expiry: %s), refreshing for credential: %s", creds.Expiry.Format(time.RFC3339), credEntry.FilePath)
		}

		// Extract client credentials from token extra data or use defaults
		clientID := config.ClientID
		clientSecret := config.ClientSecret
		if extra := creds.Extra("client_id"); extra != nil {
			if id, ok := extra.(string); ok && id != "" {
				clientID = id
			}
		}
		if extra := creds.Extra("client_secret"); extra != nil {
			if secret, ok := extra.(string); ok && secret != "" {
				clientSecret = secret
			}
		}

		oauthConfig := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		}
		tokenSource := oauthConfig.TokenSource(oauth2.NoContext, creds)
		newToken, err := tokenSource.Token()
		if err != nil {
			log.Printf("Warning: Token refresh failed for credential %s: %v", credEntry.FilePath, err)
			if creds.AccessToken == "" {
				return nil, fmt.Errorf("no access token available and refresh failed: %v", err)
			}
			// Continue with existing token as per requirement 2.4
		} else {
			log.Printf("[DEBUG] Token refreshed successfully for credential: %s (new expiry: %s)", credEntry.FilePath, newToken.Expiry.Format(time.RFC3339))
			creds = newToken
			credEntry.Token = newToken
			// Save the refreshed token back to the credential file
			if err := auth.SaveRefreshedToken(credEntry); err != nil {
				log.Printf("Warning: Failed to save refreshed token: %v", err)
			}
		}
	} else if creds.AccessToken == "" {
		return nil, fmt.Errorf("no access token available and no refresh token")
	} else {
		log.Printf("[DEBUG] Token is still valid (expiry: %s)", creds.Expiry.Format(time.RFC3339))
	}

	// Step 3: Make API request (onboarding and actual request)

	// Onboard user with selected credential
	err = auth.OnboardUser(creds, projID)
	if err != nil {
		// Check if it's a 401 error and try refreshing the token
		if strings.Contains(err.Error(), "401") && creds.RefreshToken != "" {
			log.Printf("[DEBUG] Got 401 during onboarding, forcing token refresh...")

			// Reset onboarding state since credentials are invalid
			auth.ResetOnboardingState()

			// Extract client credentials from token extra data or use defaults
			clientID := config.ClientID
			clientSecret := config.ClientSecret
			if extra := creds.Extra("client_id"); extra != nil {
				if id, ok := extra.(string); ok && id != "" {
					clientID = id
				}
			}
			if extra := creds.Extra("client_secret"); extra != nil {
				if secret, ok := extra.(string); ok && secret != "" {
					clientSecret = secret
				}
			}

			oauthConfig := &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Endpoint: oauth2.Endpoint{
					TokenURL: "https://oauth2.googleapis.com/token",
				},
			}
			tokenSource := oauthConfig.TokenSource(oauth2.NoContext, creds)
			newToken, refreshErr := tokenSource.Token()
			if refreshErr != nil {
				log.Printf("Warning: Failed to refresh token after 401: %v", refreshErr)
				return nil, fmt.Errorf("failed to onboard user: %v", err)
			}

			log.Printf("[DEBUG] Token refreshed after 401, retrying onboarding...")
			*creds = *newToken
			credEntry.Token = newToken

			// Save the refreshed token
			if saveErr := auth.SaveRefreshedToken(credEntry); saveErr != nil {
				log.Printf("Warning: Failed to save refreshed token: %v", saveErr)
			}

			// Retry onboarding with refreshed token
			if retryErr := auth.OnboardUser(creds, projID); retryErr != nil {
				return nil, fmt.Errorf("failed to onboard user after token refresh: %v", retryErr)
			}
			log.Printf("[DEBUG] Onboarding successful after token refresh")
		} else {
			return nil, fmt.Errorf("failed to onboard user: %v", err)
		}
	}

	// Build the final payload with project info
	requestData, _ := payload["request"].(map[string]interface{})
	if requestData == nil {
		requestData = make(map[string]interface{})
	}

	finalPayload := map[string]interface{}{
		"model":   payload["model"],
		"project": projID,
		"request": requestData,
	}

	// Determine the action and URL
	action := "generateContent"
	if isStreaming {
		action = "streamGenerateContent"
	}
	targetURL := config.CodeAssistEndpoint + "/v1internal:" + action
	if isStreaming {
		targetURL += "?alt=sse"
	}

	// Build request
	jsonData, err := json.Marshal(finalPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", config.GetUserAgent())

	// Send the request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	// Step 4: Return response
	var result interface{}
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
		log.Printf("[DEBUG] Usage tracked for project %s (model: %s, isPro: %v)", projID, modelName, isProModel)
	} else if resp.StatusCode != http.StatusOK {
		// Track error code for this project
		usage.GetTracker().SetErrorCode(projID, resp.StatusCode)
		log.Printf("[DEBUG] Error code %d tracked for project %s", resp.StatusCode, projID)
	}

	return result, responseErr
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

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				chunk := strings.TrimPrefix(line, "data: ")

				var obj map[string]interface{}
				if err := json.Unmarshal([]byte(chunk), &obj); err != nil {
					continue
				}

				if response, ok := obj["response"].(map[string]interface{}); ok {
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

func handleNonStreamingResponse(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Google API returned status %d: %s", resp.StatusCode, string(body))

		var errorData map[string]interface{}
		if err := json.Unmarshal(body, &errorData); err == nil {
			if errObj, ok := errorData["error"].(map[string]interface{}); ok {
				return map[string]interface{}{
					"error": errObj,
				}, nil
			}
		}

		return map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("API error: %d", resp.StatusCode),
				"code":    resp.StatusCode,
			},
		}, nil
	}

	// Parse response
	responseText := string(body)
	responseText = strings.TrimPrefix(responseText, "data: ")

	var googleAPIResponse map[string]interface{}
	if err := json.Unmarshal([]byte(responseText), &googleAPIResponse); err != nil {
		return nil, err
	}

	if standardGeminiResponse, ok := googleAPIResponse["response"].(map[string]interface{}); ok {
		return standardGeminiResponse, nil
	}

	return googleAPIResponse, nil
}

// BuildGeminiPayloadFromOpenAI builds a Gemini API payload from an OpenAI-transformed request
func BuildGeminiPayloadFromOpenAI(openaiPayload map[string]interface{}) map[string]interface{} {
	model, _ := openaiPayload["model"].(string)

	safetySettings := config.DefaultSafetySettings
	if ss, ok := openaiPayload["safetySettings"]; ok && ss != nil {
		if ssSlice, ok := ss.([]config.SafetySetting); ok {
			safetySettings = ssSlice
		}
	}

	requestData := map[string]interface{}{
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

	return map[string]interface{}{
		"model":   model,
		"request": requestData,
	}
}

// BuildGeminiPayloadFromNative builds a Gemini API payload from a native Gemini request
func BuildGeminiPayloadFromNative(nativeRequest map[string]interface{}, modelFromPath string) map[string]interface{} {
	nativeRequest["safetySettings"] = config.DefaultSafetySettings

	if _, ok := nativeRequest["generationConfig"]; !ok {
		nativeRequest["generationConfig"] = make(map[string]interface{})
	}

	return map[string]interface{}{
		"model":   config.GetBaseModelName(modelFromPath),
		"request": nativeRequest,
	}
}
