package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gcli2apigo/internal/config"

	"golang.org/x/oauth2"
)

// GetOAuthConfig returns an OAuth2 configuration using existing config constants
// This function reuses the ClientID, ClientSecret, and Scopes from the config package
// Note: Uses real Google OAuth endpoints for login (cannot be proxied)
func GetOAuthConfig(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: config.GetOAuth2Endpoint() + "/token",
		},
	}
}

// ExchangeCodeForToken exchanges an OAuth authorization code for access and refresh tokens
// This is a wrapper function that simplifies the token exchange process
func ExchangeCodeForToken(code string, redirectURL string) (*oauth2.Token, error) {
	if code == "" {
		return nil, fmt.Errorf("authorization code cannot be empty")
	}

	oauthConfig := GetOAuthConfig(redirectURL)
	ctx := context.Background()

	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	return token, nil
}

// SaveProjectCredential saves OAuth tokens to a credential file for a specific project
// The file is created in the oauth_creds directory with the format {project_id}.json
// This function ensures proper file permissions and directory structure
func SaveProjectCredential(token *oauth2.Token, projectID string, credentialsDir string) error {
	if token == nil {
		return fmt.Errorf("token cannot be nil")
	}
	if projectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}
	if credentialsDir == "" {
		credentialsDir = config.OAuthCredsFolder
	}

	// Ensure oauth_creds directory exists with 0700 permissions (owner read/write/execute only)
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Extract client credentials from token extra data or use defaults
	// Uses configurable endpoint to support reverse proxy for China users
	clientID := config.ClientID
	clientSecret := config.ClientSecret
	tokenURI := config.GetOAuth2Endpoint() + "/token"

	log.Printf("[DEBUG] OAuth token exchange - Token URI: %s", tokenURI)

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
		return fmt.Errorf("failed to marshal credential data: %w", err)
	}

	// Create file path: {project_id}.json
	filePath := filepath.Join(credentialsDir, projectID+".json")

	// Write credential file with 0600 permissions (owner read/write only)
	// This will overwrite existing files with the same project ID
	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write credential file: %w", err)
	}

	return nil
}

// RefreshToken refreshes an expired OAuth token using the refresh token
// This helper function simplifies token refresh operations
func RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	if token == nil {
		return nil, fmt.Errorf("token cannot be nil")
	}
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("refresh token is empty")
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

	// Create a minimal OAuth config for token refresh
	// Uses configurable endpoint to support reverse proxy for China users
	tokenConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: config.GetOAuth2Endpoint() + "/token",
		},
	}

	ctx := context.Background()
	newToken, err := tokenConfig.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newToken, nil
}

// ValidateToken checks if a token is valid and not expired
// Returns true if the token is valid and has not expired
func ValidateToken(token *oauth2.Token) bool {
	if token == nil {
		return false
	}
	if token.AccessToken == "" {
		return false
	}
	// Check if token has expired (with 1 minute buffer)
	return token.Expiry.After(time.Now().Add(1 * time.Minute))
}

// LoadCredentialFromFile loads OAuth credentials from a JSON file
// This helper function reads and parses a credential file into an oauth2.Token
func LoadCredentialFromFile(filePath string) (*oauth2.Token, string, error) {
	if filePath == "" {
		return nil, "", fmt.Errorf("file path cannot be empty")
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read credential file: %w", err)
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, "", fmt.Errorf("failed to parse credential file: %w", err)
	}

	// Extract required fields
	accessToken, _ := data["access_token"].(string)
	refreshToken, _ := data["refresh_token"].(string)
	tokenType, _ := data["token_type"].(string)
	projectID, _ := data["project_id"].(string)
	clientID, _ := data["client_id"].(string)
	clientSecret, _ := data["client_secret"].(string)
	tokenURI, _ := data["token_uri"].(string)

	// Parse expiry
	var expiry time.Time
	if expiryStr, ok := data["expiry"].(string); ok && expiryStr != "" {
		parsedExpiry, err := time.Parse(time.RFC3339, expiryStr)
		if err == nil {
			expiry = parsedExpiry
		}
	}

	// Create OAuth2 token
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		Expiry:       expiry,
	}

	// Store additional OAuth config data in token extra
	token = token.WithExtra(map[string]interface{}{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"token_uri":     tokenURI,
	})

	return token, projectID, nil
}
