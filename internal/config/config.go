package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// API Endpoints
const CodeAssistEndpoint = "https://cloudcode-pa.googleapis.com"

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

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
		Name:                       "models/gemini-2.5-pro-002",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Pro",
		Description:                "Latest stable version of Gemini 2.5 Pro with enhanced capabilities",
		InputTokenLimit:            2097152,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-002",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash",
		Description:                "Latest stable version of Gemini 2.5 Flash - fast and efficient",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-8b-002",
		Version:                    "002",
		DisplayName:                "Gemini 2.5 Flash 8B",
		Description:                "Lightweight version of Gemini 2.5 Flash with 8B parameters",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-2.5-flash-thinking-exp-01-21",
		Version:                    "001",
		DisplayName:                "Gemini 2.5 Flash Thinking Experimental",
		Description:                "Experimental thinking model with enhanced reasoning capabilities",
		InputTokenLimit:            32768,
		OutputTokenLimit:           32768,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-exp-1206",
		Version:                    "001",
		DisplayName:                "Gemini Experimental 1206",
		Description:                "Latest experimental Gemini model with cutting-edge features",
		InputTokenLimit:            2097152,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-exp-1121",
		Version:                    "001",
		DisplayName:                "Gemini Experimental 1121",
		Description:                "Experimental Gemini model from November 2024",
		InputTokenLimit:            2097152,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-1.5-pro-002",
		Version:                    "002",
		DisplayName:                "Gemini 1.5 Pro",
		Description:                "Latest stable version of Gemini 1.5 Pro",
		InputTokenLimit:            2097152,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-1.5-flash-002",
		Version:                    "002",
		DisplayName:                "Gemini 1.5 Flash",
		Description:                "Latest stable version of Gemini 1.5 Flash",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-1.5-flash-8b-002",
		Version:                    "002",
		DisplayName:                "Gemini 1.5 Flash 8B",
		Description:                "Lightweight version of Gemini 1.5 Flash with 8B parameters",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-pro-latest",
		Version:                    "001",
		DisplayName:                "Gemini Pro Latest",
		Description:                "Always points to the latest stable Pro model",
		InputTokenLimit:            2097152,
		OutputTokenLimit:           65536,
		SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
		Temperature:                1.0,
		MaxTemperature:             2.0,
		TopP:                       0.95,
		TopK:                       64,
	},
	{
		Name:                       "models/gemini-flash-latest",
		Version:                    "001",
		DisplayName:                "Gemini Flash Latest",
		Description:                "Always points to the latest stable Flash model",
		InputTokenLimit:            1048576,
		OutputTokenLimit:           65536,
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
