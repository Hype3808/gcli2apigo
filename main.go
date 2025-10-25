package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/banlist"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/dashboard"
	"gcli2apigo/internal/i18n"
	"gcli2apigo/internal/routes"
	"gcli2apigo/internal/usage"

	"github.com/joho/godotenv"
)

func main() {
	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Explicitly load .env file
	// This ensures settings are loaded even if autoload doesn't work
	if err := godotenv.Load(); err != nil {
		log.Printf("[WARN] No .env file found or error loading: %v", err)
		log.Printf("[INFO] Using default configuration or environment variables")
	} else {
		log.Printf("[INFO] Loaded configuration from .env file")
		// Log loaded settings (excluding password for security)
		log.Printf("[INFO] Configuration: HOST=%s, PORT=%s, MAX_RETRY_ATTEMPTS=%s",
			os.Getenv("HOST"), os.Getenv("PORT"), os.Getenv("MAX_RETRY_ATTEMPTS"))
	}

	// Reload config to pick up values from .env
	config.ReloadConfig()

	// Get server configuration
	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "7860"
	}

	// Initialize application
	log.Println("Starting Gemini proxy server...")
	if err := initializeApplication(); err != nil {
		log.Printf("Warning: Application initialization error: %v", err)
		log.Println("Server will continue but some features may not work correctly")
	}

	// Initialize dashboard handlers
	dashboardHandlers := dashboard.NewDashboardHandlers()
	oauthHandler := dashboard.NewOAuthHandler()

	// Setup routes
	mux := http.NewServeMux()

	// Health check endpoint (no auth required)
	mux.HandleFunc("/health", handleHealth)

	// Dashboard routes
	mux.HandleFunc("/dashboard/login", dashboardHandlers.HandleLogin)
	mux.HandleFunc("/dashboard/logout", dashboardHandlers.HandleLogout)
	mux.HandleFunc("/dashboard/oauth/start", dashboardHandlers.RequireAuth(oauthHandler.StartOAuthFlow))
	mux.HandleFunc("/dashboard/oauth/callback", oauthHandler.HandleCallback)
	mux.HandleFunc("/dashboard/oauth/process", oauthHandler.HandleOAuthProcess)
	mux.HandleFunc("/dashboard/api/credentials", dashboardHandlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			dashboardHandlers.HandleListCredentials(w, r)
		} else if r.Method == http.MethodDelete {
			// This handles DELETE /dashboard/api/credentials (without ID)
			http.Error(w, "Project ID required", http.StatusBadRequest)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Dashboard API route for uploading credentials
	mux.HandleFunc("/dashboard/api/credentials/upload", dashboardHandlers.RequireAuth(dashboardHandlers.HandleUploadCredentials))

	// Dashboard API routes for banning/unbanning credentials
	mux.HandleFunc("/dashboard/api/credentials/ban", dashboardHandlers.RequireAuth(dashboardHandlers.HandleBanCredential))
	mux.HandleFunc("/dashboard/api/credentials/unban", dashboardHandlers.RequireAuth(dashboardHandlers.HandleUnbanCredential))

	// Dashboard API route for stats
	mux.HandleFunc("/dashboard/api/stats", dashboardHandlers.RequireAuth(dashboardHandlers.HandleDashboardStats))

	// Dashboard API routes for language
	mux.HandleFunc("/dashboard/api/language", dashboardHandlers.HandleSetLanguage)
	mux.HandleFunc("/dashboard/api/translations", dashboardHandlers.HandleGetTranslations)

	// Dashboard API routes for settings
	mux.HandleFunc("/dashboard/api/settings", dashboardHandlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			dashboardHandlers.HandleGetSettings(w, r)
		} else if r.Method == http.MethodPost {
			dashboardHandlers.HandleSaveSettings(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Dashboard API route for deleting specific credentials
	// Pattern: /dashboard/api/credentials/{project_id}
	mux.HandleFunc("/dashboard/api/credentials/", dashboardHandlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			dashboardHandlers.HandleDeleteCredential(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// OpenAI-compatible routes
	mux.HandleFunc("/v1/chat/completions", routes.HandleChatCompletions)
	mux.HandleFunc("/v1/models", routes.HandleListModels)

	// Gemini routes
	mux.HandleFunc("/v1beta/models", routes.HandleGeminiListModels)

	// Google APIs proxy routes
	mux.HandleFunc("/googleapis", routes.HandleGoogleAPIsInfo)
	mux.HandleFunc("/googleapis/", routes.HandleGoogleAPIsProxy)

	// Catch-all for Gemini proxy and root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			handleRoot(w, r, dashboardHandlers)
		} else {
			routes.HandleGeminiProxy(w, r)
		}
	})

	// Wrap with CORS middleware
	handler := corsMiddleware(mux)

	// Setup graceful shutdown
	setupGracefulShutdown()

	// Start server
	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("Server listening on %s", addr)
	log.Println("Authentication required - Password: see .env file")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request, dashboardHandlers *dashboard.DashboardHandlers) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Content negotiation: check Accept header to determine response type
	acceptHeader := r.Header.Get("Accept")

	// If client explicitly requests JSON, return API info
	if strings.Contains(acceptHeader, "application/json") {
		handleAPIInfo(w, r)
		return
	}

	// For browser requests (text/html or no specific Accept header), show dashboard
	// Check if user is authenticated
	cookie, err := r.Cookie("session_id")
	if err != nil || cookie.Value == "" || !dashboardHandlers.GetSessionManager().ValidateSession(cookie.Value) {
		// Not authenticated, show login page
		lang := i18n.GetLanguageFromRequest(r)
		dashboard.RenderLogin(w, "", lang)
		return
	}

	// Authenticated, show dashboard
	dashboardHandlers.HandleDashboard(w, r)
}

func handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"name":        "gcli2apigo",
		"description": "OpenAI-compatible API proxy for Google's Gemini models via gemini-cli with Google APIs proxy support",
		"purpose":     "Provides both OpenAI-compatible endpoints (/v1/chat/completions), native Gemini API endpoints for accessing Google's Gemini models, and Google APIs proxy functionality",
		"version":     "1.0.0",
		"endpoints": map[string]any{
			"openai_compatible": map[string]string{
				"chat_completions": "/v1/chat/completions",
				"models":           "/v1/models",
			},
			"native_gemini": map[string]string{
				"models":   "/v1beta/models",
				"generate": "/v1beta/models/{model}/generateContent",
				"stream":   "/v1beta/models/{model}/streamGenerateContent",
			},
			"dashboard": map[string]string{
				"login":       "/dashboard/login",
				"logout":      "/dashboard/logout",
				"oauth_start": "/dashboard/oauth/start",
				"credentials": "/dashboard/api/credentials",
			},
			"googleapis_proxy": map[string]string{
				"info":    "/googleapis",
				"proxy":   "/googleapis/{api_path}",
				"example": "/googleapis/storage/v1/b",
			},
			"health": "/health",
		},
		"authentication": "Required for all endpoints except root and health",
		"repository":     "https://github.com/Hype3808/gcli2apigo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "gcli2apigo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// initializeApplication initializes all required directories and components
func initializeApplication() error {
	// Ensure oauth_creds directory exists
	credsDir := "oauth_creds"
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		log.Printf("Error: Failed to create oauth_creds directory: %v", err)
		return fmt.Errorf("failed to create oauth_creds directory: %w", err)
	}
	log.Printf("Ensured oauth_creds directory exists: %s", credsDir)

	// Initialize credential pool
	if err := auth.InitializeCredentialPool(); err != nil {
		log.Printf("Warning: Credential pool initialization error: %v", err)
		// Don't return error, just log warning
	}

	// Initialize banlist (this will create banlist.json if needed)
	banlist := banlist.GetBanList()
	log.Printf("Initialized banlist with %d banned projects", len(banlist.GetBannedProjects()))

	// Initialize usage tracker (this will create usage_stats.json if needed)
	tracker := usage.GetTracker()
	allUsage := tracker.GetAllUsage()
	log.Printf("Initialized usage tracker with %d project records", len(allUsage))

	// Check and reset usage stats if needed (handles cases where program was not running during reset time)
	tracker.CheckAndResetIfNeeded()

	// Ensure JSON files exist with empty defaults
	ensureJSONFiles(credsDir)

	return nil
}

// ensureJSONFiles creates empty JSON files if they don't exist
func ensureJSONFiles(credsDir string) {
	// Ensure banlist.json exists
	banlistPath := filepath.Join(credsDir, "banlist.json")
	if _, err := os.Stat(banlistPath); os.IsNotExist(err) {
		emptyBanlist := make(map[string]bool)
		if data, err := json.MarshalIndent(emptyBanlist, "", "  "); err == nil {
			if err := os.WriteFile(banlistPath, data, 0600); err == nil {
				log.Printf("Created empty banlist.json file")
			}
		}
	}

	// Ensure usage_stats.json exists
	usageStatsPath := filepath.Join(credsDir, "usage_stats.json")
	if _, err := os.Stat(usageStatsPath); os.IsNotExist(err) {
		emptyUsageStats := make(map[string]any)
		if data, err := json.MarshalIndent(emptyUsageStats, "", "  "); err == nil {
			if err := os.WriteFile(usageStatsPath, data, 0600); err == nil {
				log.Printf("Created empty usage_stats.json file")
			}
		}
	}
}

// setupGracefulShutdown sets up signal handlers for graceful shutdown
func setupGracefulShutdown() {
	// Note: On Windows, os.Interrupt is the only signal that works reliably
	// SIGTERM is not supported on Windows
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	go func() {
		<-sigChan
		log.Println("\nReceived interrupt signal, shutting down gracefully...")

		// Save usage stats
		if err := usage.GetTracker().Save(); err != nil {
			log.Printf("Warning: Failed to save usage stats: %v", err)
		} else {
			log.Println("Usage stats saved successfully")
		}

		// Save banlist
		if err := banlist.GetBanList().Save(); err != nil {
			log.Printf("Warning: Failed to save banlist: %v", err)
		} else {
			log.Println("Banlist saved successfully")
		}

		log.Println("Shutdown complete")
		os.Exit(0)
	}()
}
