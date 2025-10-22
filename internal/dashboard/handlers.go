package dashboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/banlist"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/i18n"
)

// DashboardHandlers manages all dashboard-related HTTP handlers
type DashboardHandlers struct {
	sessionMgr *SessionManager
}

// isSecureContext determines if the request is over HTTPS
// Checks both TLS connection and X-Forwarded-Proto header (for reverse proxy setups)
func isSecureContext(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// NewDashboardHandlers creates a new DashboardHandlers instance
func NewDashboardHandlers() *DashboardHandlers {
	return &DashboardHandlers{
		sessionMgr: NewSessionManager(),
	}
}

// HandleLogin handles password authentication for dashboard access
func (dh *DashboardHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data or JSON
	var password string
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "application/json") {
		// Handle JSON request
		var loginReq struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
			log.Printf("[ERROR] Failed to decode login request from %s: %v", r.RemoteAddr, err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		password = loginReq.Password
	} else {
		// Handle form data
		if err := r.ParseForm(); err != nil {
			log.Printf("[ERROR] Failed to parse form data from %s: %v", r.RemoteAddr, err)
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		password = r.FormValue("password")
	}

	// Validate password against GEMINI_AUTH_PASSWORD
	if password != config.GeminiAuthPassword {
		log.Printf("[WARN] Failed login attempt from %s", r.RemoteAddr)

		// Return error based on content type
		if strings.Contains(contentType, "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid password",
			})
		} else {
			// Render login page with error message
			lang := i18n.GetLanguageFromRequest(r)
			RenderLogin(w, i18n.T(lang, "login.error.invalid"), lang)
		}
		return
	}

	// Create new session
	session, err := dh.sessionMgr.CreateSession()
	if err != nil {
		log.Printf("[ERROR] Failed to create session for %s: %v", r.RemoteAddr, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set session cookie with security flags
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   86400,                   // 24 hours in seconds
		HttpOnly: true,                    // Prevents XSS attacks by making cookie inaccessible to JavaScript
		Secure:   isSecureContext(r),      // Ensures cookie is only sent over HTTPS
		SameSite: http.SameSiteStrictMode, // Prevents CSRF attacks
	})

	log.Printf("[INFO] Successful login from %s, session: %s", r.RemoteAddr, session.ID)

	// Redirect to dashboard
	if strings.Contains(contentType, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"success":  "true",
			"redirect": "/",
		})
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// HandleLogout handles session termination
func (dh *DashboardHandlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session cookie
	cookie, err := r.Cookie("session_id")
	if err == nil && cookie.Value != "" {
		// Delete session from manager
		dh.sessionMgr.DeleteSession(cookie.Value)
		log.Printf("[INFO] User logged out, session: %s", cookie.Value)
	}

	// Clear session cookie with same security flags
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,                      // Delete cookie
		HttpOnly: true,                    // Prevents XSS attacks
		Secure:   isSecureContext(r),      // Ensures cookie is only sent over HTTPS
		SameSite: http.SameSiteStrictMode, // Prevents CSRF attacks
	})

	// Redirect to login page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// requireAuth is a middleware that protects dashboard routes
// It checks for a valid session and redirects to login if not authenticated
func (dh *DashboardHandlers) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session cookie
		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value == "" {
			// No session cookie, redirect to login
			dh.redirectToLogin(w, r)
			return
		}

		// Validate session
		if !dh.sessionMgr.ValidateSession(cookie.Value) {
			// Invalid or expired session, redirect to login
			log.Printf("[WARN] Invalid or expired session: %s from %s", cookie.Value, r.RemoteAddr)
			dh.redirectToLogin(w, r)
			return
		}

		// Session is valid, proceed to next handler
		next(w, r)
	}
}

// redirectToLogin redirects to login page or returns 401 for API requests
func (dh *DashboardHandlers) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	// Check if this is an API request
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "application/json") || strings.HasPrefix(r.URL.Path, "/dashboard/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Authentication required",
		})
		return
	}

	// Render login page for browser requests
	lang := i18n.GetLanguageFromRequest(r)
	RenderLogin(w, "", lang)
}

// HandleListCredentials returns a JSON list of all stored credentials
func (dh *DashboardHandlers) HandleListCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// List all credentials
	credentials, err := ListCredentials()
	if err != nil {
		log.Printf("[ERROR] Failed to list credentials: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to list credentials",
			"details": err.Error(),
		})
		return
	}

	log.Printf("[INFO] Listed %d credentials", len(credentials))

	// Return credentials as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"credentials": credentials,
	})
}

// HandleDeleteCredential handles deletion of a credential file
func (dh *DashboardHandlers) HandleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project_id from URL path
	// Expected format: /dashboard/api/credentials/{project_id}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/dashboard/api/credentials/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Project ID is required",
		})
		return
	}

	projectID := pathParts[0]

	// Delete the credential
	err := DeleteCredential(projectID)
	if err != nil {
		log.Printf("[ERROR] Failed to delete credential for project %s: %v", projectID, err)

		// Determine appropriate status code
		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to delete credential"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Credential not found"
		} else if strings.Contains(err.Error(), "invalid project_id") {
			statusCode = http.StatusBadRequest
			errorMessage = "Invalid project ID format"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   errorMessage,
			"details": err.Error(),
		})
		return
	}

	// Reload credential pool after deletion
	if err := auth.ReloadCredentialPool(); err != nil {
		log.Printf("[WARN] Failed to reload credential pool after deletion: %v", err)
	}

	// Success response
	log.Printf("[INFO] Successfully deleted credential for project: %s", projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Credential deleted successfully",
	})
}

// HandleDashboard renders the main dashboard page with all credentials
func (dh *DashboardHandlers) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get language from request
	lang := i18n.GetLanguageFromRequest(r)

	// List all credentials
	credentials, err := ListCredentials()
	if err != nil {
		log.Printf("[ERROR] Failed to list credentials for dashboard: %v", err)
		// Render dashboard with empty credentials on error
		credentials = []CredentialInfo{}
	}

	// Render dashboard with credentials
	RenderDashboard(w, credentials, lang)
}

// HandleDashboardStats returns dashboard statistics as JSON
func (dh *DashboardHandlers) HandleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := GetDashboardStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleSetLanguage handles language switching
func (dh *DashboardHandlers) HandleSetLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Language string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate language
	lang := i18n.Language(req.Language)
	if _, exists := i18n.Translations[lang]; !exists {
		http.Error(w, "Unsupported language", http.StatusBadRequest)
		return
	}

	// Set language cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    string(lang),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		HttpOnly: false,              // Allow JavaScript access
		Secure:   isSecureContext(r),
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"language": string(lang),
	})
}

// HandleGetTranslations returns all translations for the current language
func (dh *DashboardHandlers) HandleGetTranslations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lang := i18n.GetLanguageFromRequest(r)
	translations := i18n.GetAllTranslations(lang)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"language":     string(lang),
		"translations": translations,
	})
}

// GetSessionManager returns the session manager instance
// This can be useful for testing or other components that need access
func (dh *DashboardHandlers) GetSessionManager() *SessionManager {
	return dh.sessionMgr
}

// HandleUploadCredentials handles file uploads for credentials (.json or .zip)
func (dh *DashboardHandlers) HandleUploadCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("[ERROR] Failed to parse multipart form: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to parse upload",
		})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("[ERROR] Failed to get file from form: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No file uploaded",
		})
		return
	}
	defer file.Close()

	filename := header.Filename
	log.Printf("[INFO] Received file upload: %s (%d bytes)", filename, header.Size)

	// Check file extension
	if strings.HasSuffix(strings.ToLower(filename), ".json") {
		// Handle JSON file
		count, err := HandleJSONUpload(file, filename)
		if err != nil {
			log.Printf("[ERROR] Failed to process JSON file: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		// Reload credential pool after upload
		if err := auth.ReloadCredentialPool(); err != nil {
			log.Printf("[WARN] Failed to reload credential pool after upload: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Successfully uploaded 1 credential",
			"count":   count,
		})
	} else if strings.HasSuffix(strings.ToLower(filename), ".zip") {
		// Handle ZIP file
		count, err := HandleZIPUpload(file, header.Size)
		if err != nil {
			log.Printf("[ERROR] Failed to process ZIP file: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		// Reload credential pool after upload
		if err := auth.ReloadCredentialPool(); err != nil {
			log.Printf("[WARN] Failed to reload credential pool after upload: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Successfully uploaded %d credential(s)", count),
			"count":   count,
		})
	} else {
		log.Printf("[ERROR] Unsupported file type: %s", filename)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unsupported file type. Only .json and .zip files are allowed",
		})
	}
}

// HandleBanCredential handles banning a single credential
func (dh *DashboardHandlers) HandleBanCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		ProjectIDs []string `json:"project_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if len(req.ProjectIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No project IDs provided",
		})
		return
	}

	banList := banlist.GetBanList()
	var err error

	if len(req.ProjectIDs) == 1 {
		err = banList.Ban(req.ProjectIDs[0])
	} else {
		err = banList.BanMultiple(req.ProjectIDs)
	}

	if err != nil {
		log.Printf("[ERROR] Failed to ban credentials: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to ban credentials",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully banned %d credential(s)", len(req.ProjectIDs)),
	})
}

// HandleUnbanCredential handles unbanning a single credential
func (dh *DashboardHandlers) HandleUnbanCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		ProjectIDs []string `json:"project_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if len(req.ProjectIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No project IDs provided",
		})
		return
	}

	banList := banlist.GetBanList()
	var err error

	if len(req.ProjectIDs) == 1 {
		err = banList.Unban(req.ProjectIDs[0])
	} else {
		err = banList.UnbanMultiple(req.ProjectIDs)
	}

	if err != nil {
		log.Printf("[ERROR] Failed to unban credentials: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to unban credentials",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully unbanned %d credential(s)", len(req.ProjectIDs)),
	})
}
