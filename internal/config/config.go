package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
)

// API Endpoints - configurable via environment variables
// These are now read dynamically to support runtime configuration changes
var (
	// CodeAssistEndpoint is deprecated, use GetCodeAssistEndpoint() instead
	CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com"

	// CloudResourceManagerEndpoint is deprecated, use GetCloudResourceManagerEndpoint() instead
	CloudResourceManagerEndpoint = "https://cloudresourcemanager.googleapis.com"

	// ServiceUsageEndpoint is deprecated, use GetServiceUsageEndpoint() instead
	ServiceUsageEndpoint = "https://serviceusage.googleapis.com"

	// OAuth2Endpoint is deprecated, use GetOAuth2Endpoint() instead
	OAuth2Endpoint = "https://oauth2.googleapis.com"

	// GoogleAPIsEndpoint is deprecated, use GetGoogleAPIsEndpoint() instead
	GoogleAPIsEndpoint = "https://www.googleapis.com"
)

// GetCodeAssistEndpoint returns the current Gemini Cloud Assist API endpoint
func GetCodeAssistEndpoint() string {
	return getEnvOrDefault("GEMINI_API_ENDPOINT", "https://cloudcode-pa.googleapis.com")
}

// GetCloudResourceManagerEndpoint returns the current GCP Resource Manager API endpoint
func GetCloudResourceManagerEndpoint() string {
	return getEnvOrDefault("GCP_RESOURCE_MANAGER_ENDPOINT", "https://cloudresourcemanager.googleapis.com")
}

// GetServiceUsageEndpoint returns the current GCP Service Usage API endpoint
func GetServiceUsageEndpoint() string {
	return getEnvOrDefault("GCP_SERVICE_USAGE_ENDPOINT", "https://serviceusage.googleapis.com")
}

// GetOAuth2Endpoint returns the current OAuth2 token endpoint
func GetOAuth2Endpoint() string {
	return getEnvOrDefault("OAUTH2_ENDPOINT", "https://oauth2.googleapis.com")
}

// GetGoogleAPIsEndpoint returns the current Google APIs base endpoint for proxy
func GetGoogleAPIsEndpoint() string {
	endpoint := getEnvOrDefault("GOOGLE_APIS_ENDPOINT", "https://www.googleapis.com")
	log.Printf("[DEBUG] GetGoogleAPIsEndpoint() called, returning: %s (from env: %s)", endpoint, os.Getenv("GOOGLE_APIS_ENDPOINT"))
	return endpoint
}

// Client Configuration
const CLIVersion = "0.1.5" // Match current gemini-cli version

// OAuth Configuration
const (
	ClientID     = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"
	ClientSecret = "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"
)

var Scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// File Paths
var (
	ScriptDir        string
	CredentialFile   string
	OAuthCredsFolder string
)

func init() {
	// Get the directory of the executable
	ex, err := os.Executable()
	if err != nil {
		ScriptDir = "."
	} else {
		ScriptDir = filepath.Dir(ex)
	}

	// Set credential file path
	googleAppCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if googleAppCreds == "" {
		googleAppCreds = "oauth_creds.json"
	}
	CredentialFile = filepath.Join(ScriptDir, googleAppCreds)

	// Set credentials folder path
	OAuthCredsFolder = os.Getenv("OAUTH_CREDS_FOLDER")
	if OAuthCredsFolder == "" {
		OAuthCredsFolder = filepath.Join(ScriptDir, "oauth_creds")
	}
	// Support both absolute and relative paths
	// If the path is not absolute, make it relative to ScriptDir
	if !filepath.IsAbs(OAuthCredsFolder) {
		OAuthCredsFolder = filepath.Join(ScriptDir, OAuthCredsFolder)
	}
}

// Authentication
var GeminiAuthPassword = getEnvOrDefault("GEMINI_AUTH_PASSWORD", "123456")

// Debug Logging
var DebugLoggingEnabled = os.Getenv("DEBUG_LOGGING") == "true"

// ReloadConfig reloads configuration from environment variables
// Call this after loading .env file to pick up new values
func ReloadConfig() {
	GeminiAuthPassword = getEnvOrDefault("GEMINI_AUTH_PASSWORD", "123456")
	DebugLoggingEnabled = os.Getenv("DEBUG_LOGGING") == "true"
	log.Printf("[INFO] Configuration reloaded: Password set=%v, Debug=%v",
		GeminiAuthPassword != "123456", DebugLoggingEnabled)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetMaxRetryAttempts returns the current max retry attempts setting
// This reads from environment variable each time to allow dynamic updates
func GetMaxRetryAttempts() int {
	return getEnvOrDefaultInt("MAX_RETRY_ATTEMPTS", 5)
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return DebugLoggingEnabled
}

// SafetySetting represents a safety setting for the Gemini API
type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// DefaultSafetySettings for Google API
var DefaultSafetySettings = []SafetySetting{
	{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_IMAGE_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_IMAGE_HARASSMENT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_IMAGE_HATE", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_IMAGE_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
	{Category: "HARM_CATEGORY_UNSPECIFIED", Threshold: "BLOCK_NONE"},
}

// Model represents a Gemini model
type Model struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                float64  `json:"temperature"`
	MaxTemperature             float64  `json:"maxTemperature"`
	TopP                       float64  `json:"topP"`
	TopK                       int      `json:"topK"`
}

// BaseModels (without search variants) - Updated with latest models as of October 2025
var BaseModels = []Model{
	{
		Name:                       "models/gemini-2.5-pro-preview-03-25",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Pro Preview 0325",
		Description:                "Gemini 2.5 Pro Preview 0325",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-pro-preview-06-05",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Pro Preview 0605",
		Description:                "Gemini 2.5 Pro Preview 0605",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-pro-preview-05-06",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Pro Preview 0506",
		Description:                "Gemini 2.5 Pro Preview 0506",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-pro",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Pro",
		Description:                "Gemini 2.5 Pro",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-flash-latest",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash Latest",
		Description:                "Gemini 2.5 Flash Latest",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash",
		Description:                "Gemini 2.5 Flash",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-preview-05-20",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash Preview 0520",
		Description:                "Gemini 2.5 Flash Preview 0520",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-preview-04-17",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash Preview 0417",
		Description:                "Gemini 2.5 Pro Preview 0417",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65535,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-image-preview",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash Image Preview",
		Description:                "Gemini 2.5 Flash Image Preview",
		InputTokenLimit:            32768,
		OutputTokenLimit:           32768,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-image",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash Image",
		Description:                "Gemini 2.5 Flash Image",
		InputTokenLimit:            32768,
		OutputTokenLimit:           32768,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
}

// SupportedModels includes only base models
var SupportedModels []Model

func init() {
	// Use only base models
	allModels := make([]Model, 0)
	allModels = append(allModels, BaseModels...)

	// Sort by name
	sort.Slice(allModels, func(i, j int) bool {
		return allModels[i].Name < allModels[j].Name
	})

	SupportedModels = allModels
}

// GetThinkingBudget gets the default thinking budget for a model
// Returns 1024 (minimum) to reduce thinking token usage and improve response speed
func GetThinkingBudget(modelName string) int {
	// Minimum thinking budget for all models
	return 128
}

// GetUserAgent generates User-Agent string matching gemini-cli format
func GetUserAgent() string {
	system := runtime.GOOS
	arch := runtime.GOARCH
	return "GeminiCLI/" + CLIVersion + " (" + system + "; " + arch + ")"
}

// GetPlatformString generates platform string matching gemini-cli format
func GetPlatformString() string {
	system := runtime.GOOS
	arch := runtime.GOARCH

	switch system {
	case "darwin":
		if arch == "arm64" {
			return "DARWIN_ARM64"
		}
		return "DARWIN_AMD64"
	case "linux":
		if arch == "arm64" {
			return "LINUX_ARM64"
		}
		return "LINUX_AMD64"
	case "windows":
		return "WINDOWS_AMD64"
	default:
		return "PLATFORM_UNSPECIFIED"
	}
}

// GetClientMetadata returns client metadata for API requests
func GetClientMetadata(projectID string) map[string]any {
	return map[string]any{
		"ideType":     "IDE_UNSPECIFIED",
		"platform":    GetPlatformString(),
		"pluginType":  "GEMINI",
		"duetProject": projectID,
	}
}
