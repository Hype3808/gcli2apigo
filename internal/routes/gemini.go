package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/client"
	"gcli2apigo/internal/config"
)

// HandleGeminiListModels handles native Gemini models endpoint
func HandleGeminiListModels(w http.ResponseWriter, r *http.Request) {
	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","code":401}}`, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":{"message":"Method not allowed","code":405}}`, http.StatusMethodNotAllowed)
		return
	}

	log.Println("Gemini models list requested")

	modelsResponse := map[string]interface{}{
		"models": config.SupportedModels,
	}

	log.Printf("Returning %d Gemini models", len(config.SupportedModels))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(modelsResponse)
}

// HandleGeminiListModelsV1 handles alternative models endpoint for v1 API version
func HandleGeminiListModelsV1(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/models" {
		HandleGeminiListModels(w, r)
	} else {
		http.NotFound(w, r)
	}
}

// HandleGeminiProxy handles native Gemini API proxy endpoint
func HandleGeminiProxy(w http.ResponseWriter, r *http.Request) {
	// Skip if this is a known route
	if r.URL.Path == "/" || r.URL.Path == "/health" ||
		strings.HasPrefix(r.URL.Path, "/v1/chat/completions") ||
		(r.URL.Path == "/v1/models" && r.Method == http.MethodGet) ||
		(r.URL.Path == "/v1beta/models" && r.Method == http.MethodGet) {
		http.NotFound(w, r)
		return
	}

	// Only handle Gemini API paths
	if !strings.HasPrefix(r.URL.Path, "/v1beta/") && !strings.HasPrefix(r.URL.Path, "/v1/") {
		http.NotFound(w, r)
		return
	}

	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","code":401}}`, http.StatusUnauthorized)
		return
	}

	// Get the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":{"message":"Failed to read request body","code":400}}`, http.StatusBadRequest)
		return
	}

	// Determine if this is a streaming request
	isStreaming := strings.Contains(strings.ToLower(r.URL.Path), "stream")

	// Extract model name from the path
	modelName := extractModelFromPath(r.URL.Path)

	log.Printf("Gemini proxy request: path=%s, model=%s, stream=%v", r.URL.Path, modelName, isStreaming)

	if modelName == "" {
		log.Printf("Could not extract model name from path: %s", r.URL.Path)
		http.Error(w, fmt.Sprintf(`{"error":{"message":"Could not extract model name from path: %s","code":400}}`, r.URL.Path), http.StatusBadRequest)
		return
	}

	// Parse the incoming request
	var incomingRequest map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &incomingRequest); err != nil {
			log.Printf("Invalid JSON in request body: %v", err)
			http.Error(w, `{"error":{"message":"Invalid JSON in request body","code":400}}`, http.StatusBadRequest)
			return
		}
	} else {
		incomingRequest = make(map[string]interface{})
	}

	// Build the payload for Google API
	geminiPayload := client.BuildGeminiPayloadFromNative(incomingRequest, modelName)

	// Send the request to Google API
	result, err := client.SendGeminiRequest(geminiPayload, isStreaming)
	if err != nil {
		log.Printf("Gemini proxy error: %v", err)
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Proxy error: %v", err),
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	if isStreaming {
		handleGeminiStreamingResponse(w, result)
	} else {
		handleGeminiNonStreamingResponse(w, result, modelName)
	}
}

func handleGeminiStreamingResponse(w http.ResponseWriter, result interface{}) {
	streamChan, ok := result.(chan string)
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Streaming request failed",
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("Vary", "Origin, X-Origin, Referer")
	w.Header().Set("X-XSS-Protection", "0")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Server", "ESF")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":{"message":"Streaming not supported","code":500}}`, http.StatusInternalServerError)
		return
	}

	for chunk := range streamChan {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()
	}
}

func handleGeminiNonStreamingResponse(w http.ResponseWriter, result interface{}, modelName string) {
	geminiResponse, ok := result.(map[string]interface{})
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid response from API",
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	// Check for error in response
	if errObj, ok := geminiResponse["error"]; ok {
		w.Header().Set("Content-Type", "application/json")
		if errMap, ok := errObj.(map[string]interface{}); ok {
			if code, ok := errMap["code"].(float64); ok {
				w.WriteHeader(int(code))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"error": errObj})
		return
	}

	log.Printf("Successfully processed Gemini request for model: %s", modelName)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(geminiResponse)
}

func extractModelFromPath(path string) string {
	parts := strings.Split(path, "/")

	// Look for the pattern: .../models/{model_name}/...
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelName := parts[i+1]
			// Remove any action suffix like ":streamGenerateContent" or ":generateContent"
			if idx := strings.Index(modelName, ":"); idx != -1 {
				modelName = modelName[:idx]
			}
			return modelName
		}
	}

	return ""
}
